package config

import (
	"ecos/utils/logger"
	"encoding/json"
	"errors"
	"github.com/mitchellh/copystructure"
	"io/ioutil"
	"reflect"
)

type Config struct {
}

var confMap = make(map[string]interface{})
var confPathMap = make(map[string]string)
var confDefaultMap = make(map[string]interface{})

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

	copyValueFromDefault(conf, confDefaultMap[getConfType(conf)])

	// set
	confMap[getConfType(conf)] = conf
}

func copyValueFromDefault(newConf interface{}, defaultConf interface{}) {
	rType := reflect.TypeOf(newConf)
	rVal := reflect.ValueOf(newConf)
	defaultVal := reflect.ValueOf(defaultConf)
	if rType.Kind() == reflect.Ptr {
		rType = rType.Elem()
		rVal = rVal.Elem()
		defaultVal = defaultVal.Elem()
	} else {
		return
	}

	for i := 0; i < rType.NumField(); i++ {
		f := rVal.Field(i)
		if isBlank(f) {
			defaultV := defaultVal.Field(i)
			f.Set(defaultV)
		}
	}
}

func isBlank(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return value.IsNil()
	}
	return reflect.DeepEqual(value.Interface(), reflect.Zero(value.Type()).Interface())
}

// Register a config interface to confPath
func Register(conf interface{}, confPath string) {
	confDefault, _ := copystructure.Copy(conf)
	confDefaultMap[getConfType(conf)] = confDefault
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

// WriteToPath write a config struct to confPath
// the conf don't need to Register before
func WriteToPath(conf interface{}, confPath string) error {
	b, _ := json.Marshal(conf)
	err := ioutil.WriteFile(confPath, b, 0644)
	return err
}
