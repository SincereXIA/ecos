# 开发日志


## 2022/2/21

**SincereXIA:**

- 完善 `Alaya` 相关方法和测试代码
- 完善 MemoryMetaStorage
- `Alaya` 目前已支持 multi Raft 和 meta 同步
- logger 支持设置 loglevel


## 2022/2/13

- `NodeInfoStorage` 接口支持设置 term，同 term `GroupInfo` 保证一致
- edge-node 新增两个模块 `Gaia` (盖亚) `Alaya` （阿赖耶），完成相关 proto 定义
- 增加对象元数据存储接口 `ObjectMetaStorage`
- 调研 `goCrush` 使用方法，编写了测试代码 `alaya_test.go`
- 使用 `pipline` 描述同 `PG` 节点信息
- 为 `Gaia` 增加了 Raft 模块

## 2022/1/30

- edge node 现在将生成的 uuid 以及其他配置信息持久化保存在 `$storagePath/config/edge_node.json`，
node 重启时从之前保存的文件中再次读取，保证 node 身份信息一致，避免重分配 RaftID
- sun 对已注册的 node 信息进行缓存，相同 uuid 再次注册时保证 RaftID 一致
- 增加 `utils.common` 包，自动获取指定路径的可用存储容量
- 调整 `EdgeNodeConfig`，加入 `storagePath`、 `Capacity` 配置字段
- 修复 k8s 中的 moon 注册问题（edge Docker image 错误编译成了 cloud image）

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