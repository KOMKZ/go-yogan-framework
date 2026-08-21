# Provider 函数模板

```go
// di/component_providers.go

func ProvideMyManager(i do.Injector) (*mypackage.Manager, error) {
    // 1. 获取依赖
    loader := do.MustInvoke[*config.Loader](i)
    log := do.MustInvoke[*logger.CtxZapLogger](i)
    
    // 2. 读取配置
    var cfg mypackage.Config
    if err := loader.Unmarshal("mypackage", &cfg); err != nil {
        return nil, err
    }
    cfg.ApplyDefaults()
    if err := cfg.Validate(); err != nil {
        return nil, err
    }
    
    // 3. 创建实例
    return mypackage.NewManager(cfg, log)
}
```
