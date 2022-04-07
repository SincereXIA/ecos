package config

import (
	"ecos/utils/common"
	"ecos/utils/logger"
	"encoding/json"
	"errors"
	"github.com/mitchellh/copystructure"
	"io/ioutil"
	"path"
	"reflect"
)

type Config struct {
}

var confMap = make(map[string]interface{})
var confPathMap = make(map[string]string)
var confDefaultMap = make(map[string]interface{})

// Read config from confPath and set value to conf interface
// and record it into confMap for next time get
func Read(confPath string, conf interface{}) error {
	buf, err := ioutil.ReadFile(confPath)
	if err != nil {
		logger.Warningf("load config file: %v failed: %v", confPath, err)
		return GetDefaultConf(conf)
	}
	err = json.Unmarshal(buf, conf)
	if err != nil {
		logger.Warningf("decode config file: %v failed: %v", confPath, err)
		return GetDefaultConf(conf)
	}

	copyValueFromDefault(conf, confDefaultMap[getConfType(conf)])

	// set
	confMap[getConfType(conf)] = conf
	return nil
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

func copyValue(src interface{}, dst interface{}) {
	if src == nil || dst == nil {
		return
	}
	srcType := reflect.TypeOf(src)
	srcVal := reflect.ValueOf(src)
	dstVal := reflect.ValueOf(dst)
	if srcType.Kind() == reflect.Ptr {
		srcType = srcType.Elem()
		srcVal = srcVal.Elem()
		dstVal = dstVal.Elem()
	} else {
		return
	}

	for i := 0; i < srcType.NumField(); i++ {
		f := dstVal.Field(i)
		f.Set(srcVal.Field(i))
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

// Register a config interface to confPath.
// Default Value of Config struct is set in defaultConf.
// User config json file path is confPath.
func Register(defaultConf interface{}, confPath string) {
	confDefault, _ := copystructure.Copy(defaultConf)
	key := getConfType(confDefault)
	confDefaultMap[key] = confDefault
	confMap[key] = defaultConf
	confPathMap[key] = confPath
}

func getConfType(conf interface{}) string {
	confType := reflect.TypeOf(conf)
	if confType.Kind() == reflect.Ptr {
		confType = confType.Elem()
	}
	return confType.Name()
}

// GetConf set the config value to conf interface
// the conf interface must Register() before
func GetConf(conf interface{}) error {
	if c, ok := confMap[getConfType(conf)]; ok {
		copyValue(c, conf)
		return nil
	}
	return errors.New("config not found")
}

// GetDefaultConf set the default config value to conf interface
// the conf interface must Register() before
func GetDefaultConf(conf interface{}) error {
	var ok bool
	if conf, ok = confDefaultMap[getConfType(conf)]; ok {
		return nil
	}
	return errors.New("config not found")
}

// ReadAll read all config file registered before,
// and update configMap
func ReadAll() {
	logger.Tracef("start read all config file")
	for key, conf := range confMap {
		_ = Read(confPathMap[key], conf)
	}
}

func Write(conf interface{}) error {
	if p, ok := confPathMap[getConfType(conf)]; ok {
		b, _ := json.Marshal(conf)
		err := ioutil.WriteFile(p, b, 0644)
		return err
	}
	return errors.New("config not found")
}

// WriteToPath write a config struct to confPath
// the conf don't need to Register before
func WriteToPath(conf interface{}, confPath string) error {
	err := common.InitPath(path.Dir(confPath))
	if err != nil {
		return err
	}
	b, _ := json.Marshal(conf)
	err = ioutil.WriteFile(confPath, b, 0644)
	return err
}
