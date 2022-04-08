# 会议摘要

## 2022/02/26

### 议题：

- 对接目前进度：
  - Alaya 支持元数据同步
  - 客户端文件分片和上传
  - 数据库持久化
- 接下来的工作:
  - Alaya 支持领导转让
  - edge-node 启动时初始化
  - 升级 rocksdb 版本
  - 元数据持久化
  - 客户端从 edge node 拿到 clusterInfo
  - 对象数据三节点同步
- 存在的一些问题
  - 分支提交方法和原则，rebase 和 cherry-pick
  - 再 review 一下 MR


## 2022/03/05

- 接下来的工作:
  - edge-node 启动时初始化   Z
  - 客户端从 edge node 拿到 groupInfo  X
  - 对象数据三节点同步  Q
  - rocksdb 新的序列化方法 Q
  - 客户端上传object 完整流程 X
  - 修复 ci 测试 map 竞争写的错误 Z

## 2022/3/14

- 接下来的工作
  - 三节点对象数据同步  Z
  - InfoStorage 调用方法改造 Z
  - Client 完全实现 X
  - Gogo proto 持久化 Q
  - alaya 移除 raftNode 有残留 Q
  - 实现一个接口观察当前 raft group 里面有那些节点 Q
  - Client Moon RPC X
  - Object Meta 和 BlockInfo 加上当前 term X


## 2022/3/31

- 进度对接：
  - 测试时使用随机端口号，避免冲突
  - 大重构: moon -> moon + watcher
    - Moon（Info 同步）+ Watcher （边缘节点注册 & 加入集群 & ClusterInfo 维护）
    - 什么是 info
  - 客户端支持 object 下载
    - 获取 info 的新方法: `info-agent` 
      - 不再需要 `ClientNodeInfoStorage`
  - InfoStorage 持久化进展
  
- 后续工作安排:
  - xiong:
    - S3 接口调研和实现
      - 参考：https://github.com/minio/minio/blob/master/cmd/api-router.go
      - 使用该工具检测 s3 兼容性: https://github.com/ceph/s3-tests
      - 需要兼容的接口已列出
      - 需要兼容该测试工具：https://github.com/minio/warp
  - qiutb:
    - InfoStorage 持久化
    - Moon & Alaya 状态机 **快照**, 节点重启后状态机 & raft 恢复
  - zhang:
    - 多 bucket 支持，多用户支持
    - rpc 接口鉴权

- Info:
  - 维持集群运行的信息，数据量小，更新不频繁
  - 通过 Raft 在边缘集群的所有节点上同步
  - UserInfo
  - BucketInfo
  - NodeInfo
  - ClusterInfo
  - 使用 RocksDB 存储
- Meta:
  - 对象元数据信息
  - 使用 Raft 在同 PG 的三节点同步
  - 包含对象 key、hash、分块 block ID、更新时间...
  - 使用 RocksDB 存储
- Block:
  - 对象的分块数据信息
  - 通过 PrimaryCopy / Stream / ClientCopy 分发到同 PG 的三节点中
  - 每个 block 直接以单个文件的方式存储在边缘节点文件系统中

## 2022/4/08

- 进度对接
  - zhangjh:
    - 鉴权 Interceptor
    - meta list 实现
    - mock moon
    - CLI 功能支持: get, describe, list
    - 正在实现中：Delete
    - 新工具：
      - 如何生成一个 objectID
      - 如何计算 cluster 的 pipeline
  - qiutb:
    - 快照
    - WAL
    - rocksdb storage
  - xionghc:
    - S3 进度
    - 中期报告
- 后续工作:
  - 新节点加入集群扩容
    - 新的 cluster info 发布
    - moon info 同步
    - alaya 同步，无用元数据删除 (alaya storage 可能需要大改，可能需要按 pg 分区存放)
  - 功能完善
    - 完整鉴权逻辑，用户注册，创建 bucket
  - S3 测试安排上
    - ut 里引入 aws s3 go sdk
  - 性能测试
    - 引入性能测试，防止不当功能拓展引起性能下降
