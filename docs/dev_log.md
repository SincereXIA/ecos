# 开发日志

## 2022/1/29

- 打包 docker 镜像时，在镜像内加入默认 config 文件
- CI/CD： Gitlab CI 在每次 commit 之后自动部署最新的镜像文件到 k8s 集群
  - 1 个 ecos-cloud 节点，服务域名： `ecos-cloud-dev.ecos.svc.cluster.local`
  - 5 个 ecos-edge 节点，组成边缘集群，持久化卷：`/data/ecos`
- CI/CD： 集成了单元测试
- k8s 中的 moon 注册与共识尚有问题，需要进一步修复

## 2022/1/28

- edge node 提交自身 `NodeInfo` 相关代码逻辑移动到了 `moon.Run()` 函数中
- 实现了 `utils.config` 包，用于从配置文件中读取程序配置信息
  - `utils.config` 使用 `interface` + 反射，可以从 json 文件中读取 struct 信息
  - 读取配置文件前，将数据结构和对应的配置文件路径进行注册
  - 当配置文件中信息不全时，使用注册时的默认值
  - 示例代码：
    ``` go
    // 注册、读取配置文件
    confPath := c.Path("config") // 指定配置文件路径
    conf := moonConfig.DefaultConfig // 指定默认配置
    config.Register(conf, confPath) // 注册，默认值和配置文件路径
    config.ReadAll() // 读取所有配置文件
  
    // 获取配置 struct
    var conf MoonConfig
    _ = config.GetConf(&conf) // 后续使用 GetConf 方法读取 conf 类
    // conf 中将会写入配置信息
    ```
- edge node 支持解析命令行参数，从配置文件中读取配置信息