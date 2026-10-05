package onboard

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/nothinux/certify"
	"hermannm.dev/ipfinder"
)

func (ic *InitConfig) GenerateCA() (*certify.Result, *certify.PrivateKey, error) {

	caTemplate := certify.Certificate{
		Subject: pkix.Name{
			Organization: ic.Tls.Organization,
			CommonName:   ic.Tls.CommonName,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(180 * 24 * time.Hour),
		IsCA:      true,
		KeyUsage:  x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	caCert, caKey, err := generateCertificate(caTemplate)
	if err != nil {
		return nil, nil, err
	}

	return caCert, caKey, nil
}

func generateCertificate(template certify.Certificate) (*certify.Result, *certify.PrivateKey, error) {

	key, err := certify.GetPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	ski, err := getKeyIdentifier(&key.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	template.SubjectKeyId = ski

	caCert, err := template.GetCertificate(key.PrivateKey)
	if err != nil {
		return nil, nil, err
	}
	return caCert, key, nil

}

func StoreCert(certPathMap map[string]string) error {

	for path, cert := range certPathMap {
		dirPath := filepath.Dir(path)

		if err := os.MkdirAll(dirPath, os.ModeDir|os.ModePerm); err != nil {
			return err
		}
		cleanPath := filepath.Clean(path)
		file, err := os.Create(cleanPath)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := file.Write([]byte(cert)); err != nil {
			return err
		}

	}

	return nil

}

func (ic *InitConfig) GenerateCertAndKey(caCert *certify.Result, caKey *certify.PrivateKey) (*certify.Result, *certify.PrivateKey, error) {

	ips := getIPs()

	if len(ic.Tls.IPs) > 0 {
		for _, ip := range ic.Tls.IPs {
			ips = append(ips, net.ParseIP(ip))
		}
	}
	dns := []string{"localhost", "rabbitmq"}

	if len(ic.Tls.DNS) > 0 {
		dns = append(dns, ic.Tls.DNS...)
	}

	aki, err := getKeyIdentifier(&caKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	certTemplate := certify.Certificate{
		Subject: pkix.Name{
			Organization: ic.Tls.Organization,
			CommonName:   ic.Tls.CommonName,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(180 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment | x509.KeyUsageKeyAgreement,
		ExtentedKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		Parent:           caCert.Cert,
		ParentPrivateKey: caKey.PrivateKey,
		DNSNames:         dns,
		IPAddress:        ips,
		AuthorityKeyId:   aki,
	}

	return generateCertificate(certTemplate)

}

func getKeyIdentifier(publicKey *ecdsa.PublicKey) ([]byte, error) {
	b, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	ki := sha256.Sum256(b)
	return ki[:], nil
}

func getIPs() []net.IP {

	var ips []net.IP

	publicIP, err := ipfinder.FindPublicIP(context.Background())
	if err != nil {
		fmt.Printf("Error getting public IP: %v\n", err)
		return nil
	}
	localIPs, err := ipfinder.FindLocalIPs()
	if err != nil {
		fmt.Printf("Error getting local IPs: %v\n", err)
		return nil
	}

	for _, ip := range localIPs {
		ips = append(ips, ip.Address)
	}

	ips = append(ips, publicIP)

	return ips

}

func GenerateUserAndPassword() (string, string) {

	host, _ := os.Hostname()

	plainPass := fmt.Sprintf("%s-%s", host, "accuknox")

	return host, Encode([]byte(plainPass))

}

func GetHash(input string) string {
	salt, err := generateSalt()
	if err != nil {
		log.Fatal(err)
	}
	hash := generateHashSha256(salt, input)

	hash = append(salt[:], []byte(hash[:])...)

	return Encode(hash[:])

}

func Encode(input []byte) string {
	return base64.StdEncoding.EncodeToString(input)
}

func Decode(input string) string {
	decoded, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		log.Fatal(err)
	}
	return string(decoded)
}

func generateSalt() ([4]byte, error) {
	salt := [4]byte{}
	_, err := rand.Read(salt[:])
	return salt, err
}

func generateHashSha256(salt [4]byte, password string) []byte {
	temp_hash := sha256.Sum256(append(salt[:], []byte(password)...))
	return temp_hash[:]
}

func (ic *InitConfig) GenerateOrUpdateCert(paths []string) (map[string]string, []byte, error) {

	var (
		storeData   = make(map[string]string)
		caCertBytes []byte
		configPath  = ic.TCArgs.ConfigPath
	)

	if configPath == "" {
		configPath = "/opt"
	}

	caPath := fmt.Sprintf("%s%s/%s", configPath, common.DefaultRabbitMQDir, common.DefaultCAFileName)
	certPath := fmt.Sprintf("%s%s/%s", configPath, common.DefaultRabbitMQDir, common.DefaultCertificateName)
	keyPath := fmt.Sprintf("%s%s/%s", configPath, common.DefaultRabbitMQDir, common.DefaultKeyFileName)

	if ic.Tls.Generate {
		caCert, caKey, err := ic.GenerateCA()
		if err != nil {
			return nil, nil, err
		}
		cert, key, err := ic.GenerateCertAndKey(caCert, caKey)
		if err != nil {
			return nil, nil, err
		}

		storeData[caPath] = caCert.String()
		storeData[certPath] = cert.String()
		storeData[keyPath] = key.String()

		caCertBytes = []byte(caCert.String())
	} else if len(paths) > 0 {
		for _, path := range paths {
			cert, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				return nil, nil, err
			}
			storeData[path] = string(cert)
			if filepath.Base(path) == common.DefaultCAFileName {
				caCertBytes = cert
			}
		}
	} else if ic.Tls.CaPath != "" {
		cert, err := os.ReadFile(filepath.Clean(ic.Tls.CaPath))
		if err != nil {
			return nil, nil, err
		}
		caCertBytes = cert
	} else {
		return nil, nil, fmt.Errorf("caPath is empty or generate is set to false")
	}
	return storeData, caCertBytes, nil

}
