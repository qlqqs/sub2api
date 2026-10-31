# 后端验证

候选基线与产品文件指纹见 candidate-manifest.json。Go实际工具链1.27.3，测试去除六种代理环境变量和OPENAI_API_KEY。

- go mod verify：exit0。
- 定向unit（管理员保护、渠道合并、模型缓存/OAuth、非流式协议、模型别名及图片计数）：exit0。
- 全量 go test -tags unit ./...：exit0。
- 缓存定向 go test -race -tags unit：exit0。
- go build ./... / go vet ./...：各exit0。
- 缓存负向对照：go overlay回到HEAD旧实现，只运行TestRefreshCachedOpenAIModelsRechecksFreshCache，如期exit1；对照判定exit0，证明不是恒通过测试。
- govulncheck、golangci-lint 未安装，未执行；不能以Go构建通过替代漏洞扫描或CI lint。

首次去代理测试依赖下载阶段中止(exit143)，先预取依赖后重跑。新增图片fixture首次缺output_item.done导致4个计数断言失败，补齐真实事件序列后通过；未改图片计费生产实现。保留原始日志和退出码。
