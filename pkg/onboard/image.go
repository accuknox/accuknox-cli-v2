package onboard

import (
	"fmt"
	"regexp"
	"strings"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
)

type imageOptions struct {
	customRegistry   string
	defaultRegistry  string
	defaultRepo      string
	customImage      string
	defaultImage     string
	customTag        string
	defaultTag       string
	tagPrefixToTrim  string
	tagSuffix        string
	preserveUpstream bool
}

// getImage takes image options and returns the final image
func getImage(mode VMMode, customRegistry, defaultRegistry, defaultRepo, customImage, defaultImage, customTag, defaultTag, tagPrefixToTrim, tagSuffix string, preserveUpstream bool) (string, error) {
	var registry, repo, imageName, tag string

	if customRegistry != "" {
		registry = customRegistry
	} else {
		registry = defaultRegistry
	}

	if customImage != "" {
		imageName = customImage
	} else {
		imageName = defaultImage
		if preserveUpstream {
			repo = defaultRepo
		}
	}

	if (customImage == "" && customTag == "") && tagSuffix == "" && mode == VMMode_Systemd {
		tagSuffix = cm.SystemdTagSuffix
	}

	// get image and tag
	fullImageAndTagSlice := splitLast(imageName, ":")
	if len(fullImageAndTagSlice) == 0 {
		return "", fmt.Errorf("invalid image: %s", imageName)
	} else if len(fullImageAndTagSlice) > 1 {
		tag = fullImageAndTagSlice[1]
		if strings.Contains(fullImageAndTagSlice[1], "/") {
			// check if it is custom image with registry name having a port
			// and no version
			tag = ""
			fullImageAndTagSlice[0] = imageName
		}
	}

	// get repo and image
	repoAndNameSlice := splitLast(fullImageAndTagSlice[0], "/")
	if len(repoAndNameSlice) == 0 {
		return "", fmt.Errorf("invalid image: %s", imageName)
	} else if len(repoAndNameSlice) == 1 {
		// only image name specified
		imageName = repoAndNameSlice[0]
	} else {
		// image name with possibly registry and/or repo specified
		imageName = repoAndNameSlice[1]

		// get registry and repo
		registryAndRepoSlice := splitLast(repoAndNameSlice[0], "/")
		if len(registryAndRepoSlice) == 1 {
			// maybe registry or repo specified
			if registryAndRepoSlice[0] == defaultRepo {
				if preserveUpstream {
					repo = defaultRepo
				}
			} else {
				// different registry for this image
				registry = registryAndRepoSlice[0]
			}
		} else if len(registryAndRepoSlice) > 1 {
			registry = registryAndRepoSlice[0]
			repo = registryAndRepoSlice[1]
		}
	}

	if customTag != "" {
		tag = customTag
	} else if tag == "" {
		tag = defaultTag
	}

	if tagSuffix != "" && !strings.Contains(tag, tagSuffix) {
		tag = tag + tagSuffix
	}

	if tagPrefixToTrim != "" {
		tagExp := fmt.Sprintf("%s[0-9].*\\.[0-9].*\\.[0-9].*", tagPrefixToTrim)
		exp, err := regexp.Compile(tagExp)
		if err != nil {
			return "", err
		}

		if exp.MatchString(tag) {
			tag = strings.TrimPrefix(tag, tagPrefixToTrim)
		}
	}

	var repoAndRegistry string
	if repo != "" && registry != "" {
		repoAndRegistry = registry + "/" + repo
	} else if registry != "" {
		repoAndRegistry = registry
	} else if repo != "" {
		repoAndRegistry = repo
	}

	return fmt.Sprintf("%s/%s:%s", repoAndRegistry, imageName, tag), nil
}
