# vechain 唯链客户端
## 使用介绍
 + 设置配置变量 VechainConfig
 + 设置 mysql 连接（采用xorm）
 + 运行服务 vechain.InitService(engine,config)
 + 添加上链成功后的回调函数（AddPostArtifactObserver）
 + 上链操作（幂等）
 + 查询产品的上链信息      