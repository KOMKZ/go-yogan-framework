# HTTP 集成测试

## 适用场景

应用级集成测试需要加载真实应用配置、DI、路由、中间件与生命周期，但不需要通过操作系统 TCP 端口发送请求时，使用 `testutil.NewInProcessTestServer`。

该入口会调用应用的 `RunInProcess()`：

- 执行 `Setup` 与应用 `OnSetup`；
- 创建生产 `HTTPServer` 并注册真实 Gin 路由、中间件和 Swagger；
- 进入 `Running` 状态并执行 `OnReady`；
- 不调用 `net.Listen`，因此不占用配置端口；
- 测试结束时仍须调用应用 `Shutdown()`，触发 `OnShutdown` 与 DI 资源清理。

## 应用接入

应用包装类型实现 `RunInProcess()`，先执行自身生产装配，再委托内核：

```go
func (a *UserAPI) RunInProcess() error {
    a.prepare()
    return a.Core.RunInProcess()
}
```

测试通过应用生产入口创建实例，按需在 `OnSetup` 中增加测试专属装配，然后创建服务器：

```go
server, err := testutil.NewInProcessTestServer(applicationInstance)
```

请求继续使用 `server.Engine.ServeHTTP`，所以经过真实 HTTP handler 链路；数据库、对象存储、外部 Provider 等是否真实，由应用加载的配置决定。

## 边界

- `NewTestServer` 保留原行为：完整启动并监听 TCP。
- `NewInProcessTestServer` 只覆盖 HTTP 传输方式，不替换应用配置或业务 Provider。
- 同步任务、数据 seed 和业务断言属于各应用 `test/` 包，不进入框架。
