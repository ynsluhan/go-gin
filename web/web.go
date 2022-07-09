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
	// nacos注册配置
	var namespace = conf.Cloud.Nacos.Namespace
	var cc = constant.ClientConfig{
		NamespaceId:         namespace, //namespace id
		TimeoutMs:           10000,
		NotLoadCacheAtStart: true,
		//LogDir:              "/tmp/nacos/log",
		//CacheDir:            "/tmp/nacos/cache",
		//RotateTime: "1h",
		//MaxAge:     3,
		LogLevel: "info",
	}
	// 创建服务发现客户端的另一种方式 (推荐)
	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		//panic(err)
		log.Fatalf("注销失败-1：%s", err.Error())
		os.Exit(0)
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
		//panic(err)
		log.Fatalf("注销失败-1：%s", err.Error())
		os.Exit(0)
	}
	if success {
		fmt.Println("注册成功")
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
			//panic(err)
			log.Fatalf("注销失败-1：%s", err.Error())
		}
		if res {
			fmt.Println("注销成功")
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
			//panic(err)
			log.Fatalf("注销失败-2：%s", err.Error())
		}
		if res1 {
			log.Println("注销成功")
		}
	}()
	engine.Run(fmt.Sprintf("%s:%d", conf.Server.Host, conf.Server.Port))
}