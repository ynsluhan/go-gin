package web

import (
	"bou.ke/monkey"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	config "github.com/ynsluhan/go-config"
	Thread "github.com/ynsluhan/go-new-thread"
	"log"
	"os"
	"os/signal"
	"path"
)

var c = make(chan os.Signal, 1)

var conf *config.Config

func init() {
	conf = config.GetConf()
	monkey.Patch(json.Marshal, func(v interface{}) ([]byte, error) {
		// println("via monkey patch")
		return jsoniter.Marshal(v)
	})
}

func NewEngine() *gin.Engine {
	var engine *gin.Engine
	if conf.Server.Debug {
		engine = gin.Default()
	} else {
		// 生产模式，没有控制台日志
		gin.SetMode(gin.ReleaseMode)
		// 调整cpu个数
		Thread.ConfigRuntime()
		// 开启协程
		go Thread.StatsWorker()
		engine = gin.New()
	}
	return engine
}

func Run(engine *gin.Engine) {
	// nacos地址
	sc := []constant.ServerConfig{
		{
			IpAddr: conf.Cloud.Nacos.Host,
			Port:   conf.Cloud.Nacos.Port,
		},
	}
	var logDir = conf.Cloud.Nacos.LogDir
	// 设置nacos日志
	if len(logDir) == 0 {
		getwd, _ := os.Getwd()
		logDir = path.Join(getwd, "nacos", "logs")
	}
	// nacos注册配置
	var namespace = conf.Cloud.Nacos.Namespace
	var logLevel = conf.Cloud.Nacos.LogLevel
	if len(logLevel) == 0 {
		logLevel = "info"
	}
	var cc = constant.ClientConfig{
		NamespaceId:         namespace, //namespace id
		TimeoutMs:           10000,
		NotLoadCacheAtStart: true,
		LogDir:              logDir,
		//CacheDir:            "/tmp/nacos/cache",
		//RotateTime: "1h",
		//MaxAge:     3,
		LogLevel: logLevel,
	}
	// 创建服务发现客户端的另一种方式 (推荐)
	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		log.Fatalf("[nacos] register error %s", err.Error())
	}
	// 注册端口到nacos
	success, err := namingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          conf.Server.Host,
		Port:        conf.Server.Port,
		ServiceName: "service",
		Weight:      10,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    map[string]string{"idc": "shanghai"},
		ClusterName: "DEFAULT",       // 默认值DEFAULT
		GroupName:   "DEFAULT_GROUP", // 默认值DEFAULT_GROUP
	})
	if err != nil {
		log.Fatalf("[nacos] register error %s", err.Error())
	}
	if success {
		log.Println("[nacos] register success ", success)
	}
	// 注销端口到nacos
	go func() {
		// 查看是否杀死进程
		signal.Notify(c, os.Interrupt, os.Kill)
		s := <-c
		fmt.Println("Got signal:", s)
		res, err := namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
			Ip:          conf.Server.Host,
			Port:        conf.Server.Port,
			ServiceName: conf.Server.Name,
			Ephemeral:   true,
			Cluster:     "DEFAULT",       // 默认值DEFAULT
			GroupName:   "DEFAULT_GROUP", // 默认值DEFAULT_GROUP
		})
		if err != nil {
			log.Fatalf("[nacos] deregister error %s", err.Error())
		}
		if res {
			log.Println("[nacos] deregister success ", res)
		}
		os.Exit(0)
	}()
	defer func() {
		res1, err := namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
			Ip:          conf.Server.Host,
			Port:        conf.Server.Port,
			ServiceName: "service",
			Ephemeral:   true,
			Cluster:     "DEFAULT",       // 默认值DEFAULT
			GroupName:   "DEFAULT_GROUP", // 默认值DEFAULT_GROUP
		})
		if err != nil {
			log.Fatalf("[nacos] deregister error %s", err.Error())
		}
		if res1 {
			log.Println("[nacos] deregister success ", res1)
		}
	}()
	engine.Run(fmt.Sprintf("%s:%d", conf.Server.Host, conf.Server.Port))
}
