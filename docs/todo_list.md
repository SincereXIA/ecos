# TODO List

## TaskTable

| Name                | Assign | Status   | AssignTime | Deadline |
|---------------------|--------|----------|------------|----------|
| go Crush 算法实现       | 熊胡超    | NotStart | 2022/1/23  |          |
| NodeInfo 持久化存储      | 邱天博    | Doing    | 2022/1/23  |          |
| Raft WAL & snapshot | 邱天博    | NotStart | 2022/1/23  |          |

## Todo

### Stage1: 集群共识与数据分发

- 三阶段集群共识：过去、现在、未来
- 为集群状态信息加入 Term，根据客户端 Term 编号判断集群状态是否过期
- 集群信息同步给客户端
- 实现 Crush 算法，使用 Crush 算法计算放置组
- Edge Node 与 Client 的对象数据通信和 RPC 通信
- 三个 Edge Node 构建一个 Pipeline，完成对象分块数据三副本分发
- 同一个放置组 Edge Node 构建一个 RaftGroup，实现元数据同步

### Stage2: 异常处理

- Raft 支持 Observer 机制，实现超过50节点的大规模 Raft 集群
- 数据和元数据分发过程的异常处理
- 支持节点下线，节点下线后移出 Raft，移出Edge Group
- 节点下线后重新计算存储方案
- 支持存储方案变化后原有数据迁移
- 实现 S3 存储接口

### Stage3： 云边协同
