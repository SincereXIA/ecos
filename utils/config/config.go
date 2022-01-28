package config

import (
	"ecos/utils/logger"
	"encoding/json"
	"errors"
	"io/ioutil"
	"reflect"
)

type Config struct {
}

var confMap = make(map[string]interface{})
var confPathMap = make(map[string]string)

// Read config from confPath and set value to conf interface
// and record it into confMap for next time get
func Read(confPath string, conf interface{}) {
	buf, err := ioutil.ReadFile(confPath)
	if err != nil {
		logger.Warningf("load config file: %v failed: %v", confPath, err)
		return
	}
	err = json.Unmarshal(buf, conf)
	if err != nil {
		logger.Warningf("decode config file: %v failed: %v", confPath, err)
	}
	// set
	confMap[getConfType(conf)] = conf
}

// Register a config interface to confPath
func Register(conf interface{}, confPath string) {
	confMap[reflect.TypeOf(conf).Name()] = conf
	confPathMap[reflect.TypeOf(conf).Name()] = confPath
}

func getConfType(conf interface{}) string {
	return reflect.TypeOf(conf).Name()
}

// GetConf set the config value to conf interface
// the conf interface must Register() before
func GetConf(conf interface{}) error {
	if c, ok := confMap[getConfType(conf)]; ok {
		conf = c
	}
	return errors.New("config not found")
}

// ReadAll read all config file registered before,
// and update configMap
func ReadAll() {
	logger.Infof("start read all config file")
	for key, conf := range confMap {
		Read(confPathMap[key], conf)
	}
}

func Write(conf interface{}) error {
	if path, ok := confPathMap[getConfType(conf)]; ok {
		b, _ := json.Marshal(conf)
		err := ioutil.WriteFile(path, b, 0644)
		return err
	}
	return errors.New("config not found")
}
