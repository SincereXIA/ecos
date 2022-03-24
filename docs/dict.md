#术语表

## 角色
- `cloud`：ecos 在云上部署的进程，负责协助边缘集群创建、代理读写对象等功能
- `edge-node` ecos 在边缘部署的单个节点进程
  - `moon`：集群状态共识代理，负责新节点加入集群、监控节点上下线
  - `node`：节点信息相关
    - `NodeInfo`：单个节点信息
    - `ClusterInfo`： 集群节点信息
      - `Term`：集群状态任期，同一个 Term 期间 `ClusterInfo` 保持不变
      - `LeaderInfo`： 作为领导节点的信息
      - `NodesInfo`：提供存储能力的节点信息
    - `EcosNode`：`NodeInfo`的包装，包含CRUSH算法相关信息

## 概念

- `bucket`：对象库，用户可以创建 `bucket` 管理一组对象
- `object`: 对象，客户端写入数据的最小单位
  - `meta`: 对象元数据信息
    - `metadata`：用户自定义的元数据信息
  - `block`: 对象分块，`object` 最终落盘存储的最小单位，大文件将会分成多个 `block` 存储
    - `chunk`: 数据流块，对象在网络中传输时的最小单位
- `pipeline`: 数据流组，对象数据多副本放置过程的描述
  - 包含了副本存放的节点信息、数据流的流动顺序
- `PlaceGroup、PG`：放置组
  - 当客户端提交对象时，通过 `crush` 算法为每个 `block` 计算出一个 `PG`
  - 一个 `edge-node` 会属于多个 `PG`
  - `edge-node` 上数据最终的存储路径为 `/ecos/container/pgid/blockid`

## 一些示例

对于对象存储的各层级来说

```
key: "path/to/file"
objectID: "volume/bucket/path/to/file"
blockID: "fdshf289rufdhjfajklshjf2"
```