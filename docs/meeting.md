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