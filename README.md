# Indie Studio 更新源

此仓库保存 Indie Studio 的签名更新清单、安装包与源码快照。

## 下载

- 稳定版安装器：https://github.com/Tangerine-Living/indie-studio/releases/latest/download/indie-studio-0.1.0-installer.exe
- 稳定版免安装：https://github.com/Tangerine-Living/indie-studio/releases/latest/download/indie-studio-0.1.0-portable.exe
- 稳定版源码：https://github.com/Tangerine-Living/indie-studio/releases/latest/download/indie-studio-0.1.0-source.zip
- 源码分支：`stable` 与 `dev`（当前两者内容一致，只含项目代码与默认设置，不含用户配置）

## 更新清单

- 稳定版：`stable/latest.json` 与 `stable/latest.json.sig`
- 开发版：`dev/latest.json` 与 `dev/latest.json.sig`

开发版当前与稳定版同为 0.1.0。清单由应用仓库的 `scripts/publish-update.ps1` 生成并提交，请勿手动编辑。应用内置 Ed25519 公钥，只接受本仓库签名的清单。
