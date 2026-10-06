# Embedded in the generated Bash script after the credential assignments.
usage() {
    printf 'Usage: %s {docker|docker-compose|systemd} [container-name]\n' "$0"
    printf 'The optional container name is supported for Docker modes only.\n'
}
case ${1:-} in
    docker|docker-compose|systemd) export RMQ_MODE=$1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
esac
if (( $# > 1 )) || [[ $RMQ_MODE == systemd && $# != 0 ]]; then
    usage >&2
    exit 2
fi
if [[ $EUID -ne 0 ]]; then exec sudo -- bash "$0" "$RMQ_MODE" "$@"; fi
umask 077

die() { printf '%s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null || die "Required command is missing: $1"; }
inspect_list_count() {
    local field=$1 result
    # Docker reports unset lists/maps as null in some Engine versions. Calling
    # len directly on those values fails instead of returning zero.
    if ! result=$(docker inspect --format "{{if .HostConfig.$field}}{{len .HostConfig.$field}}{{else}}0{{end}}" "$container"); then
        die "Cannot inspect Docker $field settings."
    fi
    printf '%s\n' "$result"
}
confirm() {
    local answer
    printf '%s Continue? [y/N] ' "$*" >/dev/tty
    read -r answer </dev/tty
    [[ $answer == y || $answer == Y || $answer == yes ]] || die 'Cancelled.'
}
prompt() {
    printf '%s' "$1" >/dev/tty
    read -r REPLY </dev/tty
}
prepare_systemd_state() {
    # Definitions export is performed by the broker, even when rabbitmqctl is
    # invoked as root. Give its service account access to this private directory.
    chown rabbitmq:rabbitmq "$state"
    chmod 750 "$state"
}

need awk
need mktemp
if [[ ! $RMQ_PORT =~ ^[0-9]+$ ]] || (( RMQ_PORT < 1 || RMQ_PORT > 65535 )); then
    die 'Invalid broker port.'
fi

state=$(mktemp -d /var/lib/rabbitmq-setup.XXXXXXXX)
chmod 755 "$state"
container=''
replaced=false
created=false
stopped=false
systemd_changed=false
compose_changed=false
finished=false

ctl() {
    if [[ $RMQ_MODE == systemd ]]; then rabbitmqctl "$@"
    else docker exec "$container" rabbitmqctl "$@"; fi
}
diagnostics() {
    if [[ $RMQ_MODE == systemd ]]; then rabbitmq-diagnostics "$@"
    else docker exec "$container" rabbitmq-diagnostics "$@"; fi
}
wait_ready() {
    local count
    # ping checks only the Erlang runtime. The rabbit application and vhosts
    # must be started before authentication or definitions import can succeed.
    for ((count=0; count<60; count++)); do
        if diagnostics -q --timeout 5 check_running >/dev/null 2>&1 &&
           diagnostics -q --timeout 5 check_virtual_hosts >/dev/null 2>&1; then
            return
        fi
        sleep 2
    done
    return 1
}
ensure_user() {
    local users
    users=$(ctl -q list_users)
    if awk -F '\t' -v name="$RMQ_USERNAME" '$1 == name {found=1} END {exit !found}' <<<"$users"; then
        ctl change_password "$RMQ_USERNAME" "$RMQ_PASSWORD"
    else
        ctl add_user "$RMQ_USERNAME" "$RMQ_PASSWORD"
    fi
    if ! ctl -q list_vhosts name | awk -F '\t' '$1 == "/" {found=1} END {exit !found}'; then
        ctl add_vhost /
    fi
    ctl set_permissions -p / "$RMQ_USERNAME" '.*' '.*' '.*'
}
rollback() {
    local result=$?
    trap - EXIT
    if [[ $finished != true && $result != 0 ]]; then
        printf '\nSetup failed. Backups: %s\n' "$state" >&2
        if [[ $replaced == true ]]; then
            if [[ $created == true ]]; then docker rm -f "$container" >/dev/null 2>&1 || true; fi
            docker rename "$original" "$container" || true
            docker start "$container" || true
        elif [[ $compose_changed == true ]]; then
            "${compose[@]}" up -d --no-deps "$service" || true
        elif [[ $stopped == true ]]; then
            docker start "$container" || true
        elif [[ $systemd_changed == true ]]; then
            if [[ $had_config == true ]]; then cp -p "$state/rabbitmq.conf.before" /etc/rabbitmq/rabbitmq.conf
            else rm -f /etc/rabbitmq/rabbitmq.conf; fi
            systemctl restart rabbitmq-server || true
        fi
        if [[ -f $state/definitions.before.json ]]; then
            if wait_ready; then
                if [[ $RMQ_MODE == systemd ]]; then
                    ctl import_definitions "$state/definitions.before.json" || true
                else
                    if docker cp "$state/definitions.before.json" "$container:/tmp/rabbitmq-setup-restore.json"; then
                        ctl import_definitions /tmp/rabbitmq-setup-restore.json || true
                    fi
                fi
            else
                printf 'The original RabbitMQ application is not ready; definitions were not restored. Backup: %s/definitions.before.json\n' "$state" >&2
            fi
        fi
    fi
    exit "$result"
}
trap rollback EXIT

# Preserve unrelated configuration; transport, credentials and boot definitions
# are managed by this script and backed up before they are changed.
make_config() {
    local listener_port=$1 definitions=$2
    awk '!/^[[:space:]]*(listeners\.(tcp|ssl)|ssl_options\.|loopback_users|load_definitions|management\.load_definitions|definitions\.)/' \
        "$state/rabbitmq.conf.before" >"$state/rabbitmq.conf"
    if [[ $RMQ_TLS == true ]]; then
        printf 'listeners.tcp = none\nlisteners.ssl.default = %s\n' "$listener_port" >>"$state/rabbitmq.conf"
        local key path
        for key in cacertfile certfile keyfile; do
            path=$(awk -F '=' -v key="ssl_options.$key" '{k=$1; gsub(/^[ \t]+|[ \t]+$/, "", k); if(k==key){v=$2; gsub(/^[ \t]+|[ \t]+$/, "", v); print v; exit}}' "$state/rabbitmq.conf.before")
            if [[ -z $path ]]; then
                case $key in
                    cacertfile) path=/etc/ssl/ca_certificate.pem ;;
                    certfile) path=/etc/ssl/server_certificate.pem ;;
                    keyfile) path=/etc/ssl/server_key.pem ;;
                esac
            fi
            if [[ $RMQ_MODE == systemd ]]; then
                [[ -r $path ]] || { prompt "Broker TLS $key path: "; path=$REPLY; }
                [[ -r $path ]] || die "TLS file is unreadable: $path"
            else
                docker exec "$container" test -r "$path" || { prompt "Broker TLS $key path inside container: "; path=$REPLY; }
                docker exec "$container" test -r "$path" || die "TLS file is unreadable: $path"
            fi
            [[ $path != *'#'* && $path != *$'\n'* ]] || die 'Unsupported TLS path.'
            printf 'ssl_options.%s = %s\n' "$key" "$path" >>"$state/rabbitmq.conf"
        done
        printf 'ssl_options.verify = verify_peer\nssl_options.fail_if_no_peer_cert = false\n' >>"$state/rabbitmq.conf"
    else
        printf 'listeners.tcp.default = %s\n' "$listener_port" >>"$state/rabbitmq.conf"
    fi
    printf 'loopback_users.guest = true\nload_definitions = %s\n' "$definitions" >>"$state/rabbitmq.conf"
}

if [[ $RMQ_MODE == systemd ]]; then
    need rabbitmqctl
    need rabbitmq-diagnostics
    need systemctl
    systemctl is-active --quiet rabbitmq-server || die 'rabbitmq-server is not running.'
    [[ ! -f /etc/rabbitmq/advanced.config ]] || die 'Custom advanced.config needs manual review.'
    wait_ready || die 'The existing RabbitMQ application is not running.'
    prepare_systemd_state
    had_config=false
    if [[ -f /etc/rabbitmq/rabbitmq.conf ]]; then
        cp -p /etc/rabbitmq/rabbitmq.conf "$state/rabbitmq.conf.before"
        had_config=true
    else
        : >"$state/rabbitmq.conf.before"
    fi
    make_config "$RMQ_PORT" "$state/definitions.json"
    ctl export_definitions "$state/definitions.before.json"
    chown rabbitmq:rabbitmq "$state/definitions.before.json"
    chmod 600 "$state/definitions.before.json"
    confirm 'RabbitMQ will be configured and restarted; brief downtime is required.'
    ensure_user
    ctl export_definitions "$state/definitions.json"
    chown rabbitmq:rabbitmq "$state/definitions.json" "$state/rabbitmq.conf"
    chmod 600 "$state/definitions.json"
    chmod 640 "$state/rabbitmq.conf"
    mkdir -p /etc/rabbitmq
    systemd_changed=true
    cp -p "$state/rabbitmq.conf" /etc/rabbitmq/rabbitmq.conf
    systemctl restart rabbitmq-server
else
    need docker
    # Discovery happens on the broker host at execution time, including remote brokers.
    candidates=()
    while IFS= read -r name; do
        [[ -n $name ]] || continue
        project=$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project"}}' "$name")
        if [[ $RMQ_MODE == docker-compose ]]; then [[ -n $project && $project != '<no value>' ]] || continue
        else [[ -z $project || $project == '<no value>' ]] || continue; fi
        if docker exec "$name" rabbitmq-diagnostics -q ping >/dev/null 2>&1; then candidates+=("$name"); fi
    done < <(docker ps --format '{{.Names}}')
    container=${1:-}
    if [[ -z $container && ${#candidates[@]} == 1 ]]; then container=${candidates[0]}; fi
    if [[ -z $container ]]; then
        printf 'Running RabbitMQ containers: %s\n' "${candidates[*]:-none}"
        prompt 'RabbitMQ container name: '
        container=$REPLY
    fi
    found=false
    for name in "${candidates[@]}"; do [[ $name != "$container" ]] || found=true; done
    [[ $found == true ]] || die 'No running RabbitMQ container of the selected installation type.'
    wait_ready || die 'The existing RabbitMQ application is not running; check container logs.'
    docker inspect "$container" >"$state/container.before.json"
    if ! docker cp "$container:/etc/rabbitmq/rabbitmq.conf" "$state/rabbitmq.conf.before" 2>/dev/null; then
        if docker exec "$container" test -e /etc/rabbitmq/rabbitmq.conf; then die 'Cannot back up RabbitMQ configuration.'; fi
        : >"$state/rabbitmq.conf.before"
    fi
    if docker exec "$container" test -e /etc/rabbitmq/advanced.config; then die 'Custom advanced.config needs manual review.'; fi
    env_values=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$container")
    if [[ $env_values == *RABBITMQ_MNESIA_DIR=* || $env_values == *RABBITMQ_MNESIA_BASE=* || $env_values == *RABBITMQ_CONFIG_FILE=* ]]; then
        die 'Custom RabbitMQ configuration/data paths need manual review.'
    fi
    hostname=$(docker inspect --format '{{.Config.Hostname}}' "$container")
    uid=$(docker exec "$container" id -u rabbitmq)
    gid=$(docker exec "$container" id -g rabbitmq)
    mounts=$(docker inspect --format '{{range .Mounts}}{{printf "%s|%s|%s|%s|%t\n" .Type .Name .Source .Destination .RW}}{{end}}' "$container")
    network=$(docker inspect --format '{{.HostConfig.NetworkMode}}' "$container")
    ports=$(docker inspect --format '{{range $port, $bindings := .HostConfig.PortBindings}}{{range $bindings}}{{printf "%s|%s|%s\n" .HostIp .HostPort $port}}{{end}}{{end}}' "$container")
    [[ $network == host || $ports == *'|5672/tcp'* ]] || die 'Publish the broker container port 5672 before setup.'
    while IFS='|' read -r type volume source target writable; do
        [[ -n $type ]] || continue
        [[ $type == bind || $type == volume ]] || die 'Custom mount types need manual review.'
        [[ $target != /var/lib/rabbitmq/* ]] || die 'Split data mounts need manual review.'
        [[ $target != /var/lib/rabbitmq || $writable == true ]] || die 'RabbitMQ data mount must be writable.'
        [[ $source != *','* && $source != *$'\n'* ]] || die 'Unsupported mount path.'
    done <<<"$mounts"
    if [[ $RMQ_MODE == docker-compose ]]; then
        docker compose version >/dev/null
        service=$(docker inspect --format '{{index .Config.Labels "com.docker.compose.service"}}' "$container")
        project=$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project"}}' "$container")
        workdir=$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$container")
        files=$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project.config_files"}}' "$container")
        [[ $service =~ ^[a-zA-Z0-9_.-]+$ && -d $workdir ]] || die 'Original Compose project metadata is required.'
        compose=(docker compose --project-directory "$workdir" -p "$project")
        IFS=',' read -r -a compose_files <<<"$files"
        for file in "${compose_files[@]}"; do [[ -f $file ]] || die "Missing Compose file: $file"; compose+=(-f "$file"); done
    else
        # A committed image preserves environment, entrypoint, command and container
        # filesystem; the volume/network/port settings are copied below.
        [[ $(docker inspect --format '{{if .NetworkSettings.Networks}}{{len .NetworkSettings.Networks}}{{else}}0{{end}}' "$container") == 1 ]] || die 'Multiple Docker networks need manual review.'
        [[ $(docker inspect --format '{{.HostConfig.Privileged}}' "$container") == false ]] || die 'Privileged containers need manual review.'
        [[ $(docker inspect --format '{{.HostConfig.AutoRemove}}' "$container") == false ]] || die 'Auto-remove containers need manual migration.'
        [[ $(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "$container") == false ]] || die 'Read-only root containers need manual review.'
        for field in CapAdd CapDrop SecurityOpt Devices DeviceRequests Ulimits Dns DnsSearch Tmpfs Links VolumesFrom; do
            count=$(inspect_list_count "$field")
            [[ $count == 0 ]] || die "Custom $field settings need manual review."
        done
        for field in Memory MemorySwap MemoryReservation NanoCpus CpuShares CpuQuota CpuPeriod; do
            [[ $(docker inspect --format "{{.HostConfig.$field}}" "$container") == 0 ]] || die "Custom $field resource limits need manual review."
        done
        [[ -z $(docker inspect --format '{{.HostConfig.CpusetCpus}}' "$container") ]] || die 'Custom CPU affinity needs manual review.'
        static_ip=$(docker inspect --format '{{range .NetworkSettings.Networks}}{{if .IPAMConfig}}{{.IPAMConfig.IPv4Address}}{{.IPAMConfig.IPv6Address}}{{end}}{{end}}' "$container")
        [[ -z $static_ip ]] || die 'Static container IPs need manual review.'
    fi
    make_config 5672 /etc/rabbitmq/knoxctl-definitions.json
    ctl export_definitions /tmp/rabbitmq-setup-before.json
    docker cp "$container:/tmp/rabbitmq-setup-before.json" "$state/definitions.before.json"
    chmod 600 "$state/definitions.before.json"
    confirm "RabbitMQ container $container will be recreated with persistent storage; brief downtime is required."
    ensure_user
    ctl export_definitions /tmp/rabbitmq-setup-definitions.json
    docker cp "$container:/tmp/rabbitmq-setup-definitions.json" "$state/definitions.json"
    chown "$uid:$gid" "$state/definitions.json" "$state/rabbitmq.conf"
    chmod 600 "$state/definitions.json"
    chmod 640 "$state/rabbitmq.conf"
    stopped=true
    docker stop "$container"
    data_source=''
    data_type=bind
    while IFS='|' read -r type volume source target writable; do
        if [[ $target == /var/lib/rabbitmq ]]; then
            data_type=$type
            if [[ $type == volume ]]; then data_source=$volume; else data_source=$source; fi
        fi
    done <<<"$mounts"
    if [[ -z $data_source ]]; then
        data_source=$state/data
        mkdir "$data_source"
        docker cp -a "$container:/var/lib/rabbitmq/." "$data_source"
        chown "$uid:$gid" "$data_source"
    fi
    if [[ $RMQ_MODE == docker-compose ]]; then
        # A JSON string is also a valid YAML quoted scalar. Avoid external parsers.
        yaml_quote() { local value=$1; value=${value//\\/\\\\}; value=${value//\"/\\\"}; printf '"%s"' "$value"; }
        override=$state/compose-persistent.yaml
        {
            printf 'services:\n  %s:\n    hostname: ' "$service"; yaml_quote "$hostname"
            printf '\n    restart: unless-stopped\n    volumes:\n'
            if [[ $data_type == volume ]]; then
                printf '      - type: volume\n        source: knoxctl_rabbitmq_data\n        target: /var/lib/rabbitmq\n'
            else
                printf '      - type: bind\n        source: '; yaml_quote "$data_source"; printf '\n        target: /var/lib/rabbitmq\n'
            fi
            for spec in 'rabbitmq.conf|/etc/rabbitmq/rabbitmq.conf' 'definitions.json|/etc/rabbitmq/knoxctl-definitions.json'; do
                IFS='|' read -r file target <<<"$spec"
                printf '      - type: bind\n        source: '; yaml_quote "$state/$file"
                printf '\n        target: %s\n        read_only: true\n' "$target"
            done
            if [[ $data_type == volume ]]; then
                printf 'volumes:\n  knoxctl_rabbitmq_data:\n    external: true\n    name: '; yaml_quote "$data_source"; printf '\n'
            fi
        } >"$override"
        chmod 644 "$override"
        compose_changed=true
        "${compose[@]}" -f "$override" up -d --no-deps "$service"
        { printf '#!/usr/bin/env bash\nset -euo pipefail\n'; printf '%q ' "${compose[@]}" -f "$override" up -d --no-deps "$service"; printf '\n'; } >"$state/compose-up.sh"
        chmod 755 "$state/compose-up.sh"
    else
        image=$(docker commit "$container")
        original=$container-before-setup-$(date +%s)
        docker rename "$container" "$original"
        replaced=true
        create=(docker create --name "$container" --hostname "$hostname" --restart unless-stopped --network "$network")
        if [[ $network != host && $network != bridge && $network != default && $network != none ]]; then
            while IFS= read -r alias; do [[ -z $alias ]] || create+=(--network-alias "$alias"); done \
                < <(docker inspect --format '{{range .NetworkSettings.Networks}}{{range .Aliases}}{{println .}}{{end}}{{end}}' "$original")
        fi
        while IFS='|' read -r type volume source target writable; do
            [[ -n $type ]] || continue
            case $target in /var/lib/rabbitmq|/etc/rabbitmq/rabbitmq.conf|/etc/rabbitmq/knoxctl-definitions.json) continue ;; esac
            if [[ $type == volume ]]; then source=$volume; fi
            spec="type=$type,source=$source,target=$target"
            [[ $writable == true ]] || spec+=,readonly
            create+=(--mount "$spec")
        done <<<"$mounts"
        create+=(--mount "type=$data_type,source=$data_source,target=/var/lib/rabbitmq")
        create+=(--mount "type=bind,source=$state/rabbitmq.conf,target=/etc/rabbitmq/rabbitmq.conf,readonly")
        create+=(--mount "type=bind,source=$state/definitions.json,target=/etc/rabbitmq/knoxctl-definitions.json,readonly")
        while IFS='|' read -r ip host_port container_port; do
            [[ -n $host_port ]] || continue
            if [[ -n $ip ]]; then
                [[ $ip != *:* ]] || ip="[$ip]"
                create+=(-p "$ip:$host_port:$container_port")
            else create+=(-p "$host_port:$container_port"); fi
        done <<<"$ports"
        pid=$(docker inspect --format '{{.HostConfig.PidMode}}' "$original")
        [[ -z $pid ]] || create+=(--pid "$pid")
        ipc=$(docker inspect --format '{{.HostConfig.IpcMode}}' "$original")
        [[ -z $ipc ]] || create+=(--ipc "$ipc")
        log_driver=$(docker inspect --format '{{.HostConfig.LogConfig.Type}}' "$original")
        [[ -z $log_driver ]] || create+=(--log-driver "$log_driver")
        while IFS= read -r option; do [[ -z $option ]] || create+=(--log-opt "$option"); done \
            < <(docker inspect --format '{{range $key, $value := .HostConfig.LogConfig.Config}}{{printf "%s=%s\n" $key $value}}{{end}}' "$original")
        while IFS= read -r extra; do [[ -z $extra ]] || create+=(--add-host "$extra"); done \
            < <(docker inspect --format '{{range .HostConfig.ExtraHosts}}{{println .}}{{end}}' "$original")
        create+=("$image")
        "${create[@]}"
        created=true
        docker start "$container"
        { printf '#!/usr/bin/env bash\nset -euo pipefail\n'; printf '%q ' "${create[@]}"; printf '\n'; printf 'docker start %q\n' "$container"; } >"$state/docker-create.sh"
        chmod 755 "$state/docker-create.sh"
    fi
fi

if ! wait_ready; then
    diagnostics -q status >&2 || true
    if [[ $RMQ_MODE != systemd ]]; then docker logs --tail 80 "$container" >&2 || true; fi
    die 'RabbitMQ application failed to become ready; see the broker logs above.'
fi
ctl authenticate_user "$RMQ_USERNAME" "$RMQ_PASSWORD"
ctl list_user_permissions "$RMQ_USERNAME"
finished=true
printf '\nRabbitMQ setup complete. Persistent configuration and backups: %s\n' "$state"
if [[ $RMQ_MODE == docker-compose ]]; then
    printf 'For future Compose recreation, keep the generated override and use: %s/compose-up.sh\n' "$state"
elif [[ $RMQ_MODE == docker ]]; then
    printf 'Original container retained for rollback: %s\nRecreation command saved to: %s/docker-create.sh\n' "$original" "$state"
fi
printf 'Worker authentication: --auth=%s\n' "$RMQ_AUTH"
printf 'Allow the broker AMQP port through your firewall/security group for worker connections.\n'
