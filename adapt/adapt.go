package adapt

type OnDemandConverter struct{}

// 按需转换器持有阶段 1 缓存并返回修复后的整根区块，viewer 将结果适配成 dispatcher.Section。
// 构造函数需要接收源区块读取入口和 fixer，缓存由转换器管理。
