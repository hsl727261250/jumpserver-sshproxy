package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name               string `yaml:"NAME"`
	CoreHost           string `yaml:"CORE_HOST"`
	BootstrapToken     string `yaml:"BOOTSTRAP_TOKEN"`
	LogLevel           string `yaml:"LOG_LEVEL"`
	HttpRequestTimeout int    `yaml:"HTTP_REQUEST_TIMEOUT"`
	TerminalType       string `yaml:"TERMINAL_TYPE"`
	ForwardDialTimeout int    `yaml:"FORWARD_DIAL_TIMEOUT"`
	MaxIdleMinutes     int    `yaml:"MAX_IDLE_MINUTES"`
	MaxSessionHours    int    `yaml:"MAX_SESSION_HOURS"`
	AssetID            string `yaml:"ASSET_ID"`

	RootPath          string `yaml:"-"`
	DataFolderPath    string `yaml:"-"`
	LogDirPath        string `yaml:"-"`
	KeyFolderPath     string `yaml:"-"`
	AccessKeyFilePath string `yaml:"-"`
	HostKeyFilePath   string `yaml:"-"`
}

var GlobalConfig *Config

func GetConf() Config {
	if GlobalConfig == nil {
		return getDefaultConfig()
	}
	return *GlobalConfig
}

func Setup(configPath string) {
	conf := getDefaultConfig()
	loadConfigFromFile(configPath, &conf)
	applyCoreEnv(&conf)
	GlobalConfig = &conf
	log.Printf("Config loaded: name=%q bind=%s ssh_port=%s core=%s\n",
		conf.Name, ListenHost, ListenPort, conf.CoreHost)
}

func getDefaultConfig() Config {
	rootPath := getPwdDirPath()
	dataFolderPath := filepath.Join(rootPath, "data")
	logDirPath := filepath.Join(dataFolderPath, "logs")
	keyFolderPath := filepath.Join(dataFolderPath, "keys")
	for _, folder := range []string{dataFolderPath, logDirPath, keyFolderPath} {
		if err := os.MkdirAll(folder, 0700); err != nil {
			log.Fatalf("Create folder failed: %s", err)
		}
	}
	return Config{
		Name:               getDefaultName(),
		CoreHost:           "http://127.0.0.1:8080",
		LogLevel:           "INFO",
		HttpRequestTimeout: 30,
		TerminalType:       "koko",
		ForwardDialTimeout: 15,
		MaxIdleMinutes:     30,
		MaxSessionHours:    24,
		RootPath:           rootPath,
		DataFolderPath:     dataFolderPath,
		LogDirPath:         logDirPath,
		KeyFolderPath:      keyFolderPath,
		AccessKeyFilePath:  filepath.Join(keyFolderPath, ".access_key"),
		HostKeyFilePath:    filepath.Join(keyFolderPath, "ssh_host_ed25519_key"),
	}
}

func applyCoreEnv(conf *Config) {
	if value, ok := os.LookupEnv("CORE_HOST"); ok && value != "" {
		conf.CoreHost = value
	}
	if value, ok := os.LookupEnv("BOOTSTRAP_TOKEN"); ok && value != "" {
		conf.BootstrapToken = value
	}
}

func loadConfigFromFile(path string, conf *Config) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Config file %s not found, using defaults and environment\n", path)
			return
		}
		log.Fatalf("Load config from %s failed: %s", path, err)
	}
	if err = yaml.Unmarshal(content, conf); err != nil {
		log.Fatalf("Load config from %s failed: %s", path, err)
	}
	log.Printf("Load config from %s success\n", path)
}

func getPwdDirPath() string {
	rootPath, err := os.Getwd()
	if err != nil {
		return ""
	}
	return rootPath
}

const (
	ListenHost = "0.0.0.0"
	ListenPort = "1024"
)

const (
	prefixName        = "[SSHProxy]-"
	hostEnvKey        = "SERVER_HOSTNAME"
	defaultNameMaxLen = 128
)

func getDefaultName() string {
	hostname, _ := os.Hostname()
	hostname = fmt.Sprintf("%s-%s", hostname, randomStr(7))
	if serverHostname, ok := os.LookupEnv(hostEnvKey); ok && serverHostname != "" {
		hostname = fmt.Sprintf("%s-%s", serverHostname, hostname)
	}
	hostRune := []rune(prefixName + hostname)
	if len(hostRune) <= defaultNameMaxLen {
		return string(hostRune)
	}
	name := make([]rune, defaultNameMaxLen)
	index := defaultNameMaxLen / 2
	copy(name[:index], hostRune[:index])
	start := len(hostRune) - index
	copy(name[index:], hostRune[start:])
	return string(name)
}

func randomStr(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "0000000"
	}
	encoded := hex.EncodeToString(buf)
	if len(encoded) > n {
		return encoded[:n]
	}
	return encoded
}
