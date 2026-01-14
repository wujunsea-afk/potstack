# PotStack PPK 集成状态报告

## 摘要
成功完成 PotPacker 与 PotStack Loader 的集成，以支持新的安全 PPK 包格式。系统现已采用 ED25519 签名进行完整性校验，并使用 Zip 压缩以提升移植性。

## 关键变更

### 1. PotPacker (Rust 工具)
- **格式**: 内部压缩格式从 7z 更改为 Zip (Deflate)，以获得与 Go 标准库更好的兼容性。
- **功能**:
    - `keygen`: 生成 ED25519 密钥对 (`.key`, `.pub`)。
    - `pack`: 将目录压缩为带签名的 `.ppk` 格式。
    - `verify`: 使用公钥验证 `.ppk` 文件的完整性。
- **依赖**: 将 `sevenz-rust` 替换为 `zip`。

### 2. PotStack Loader (Go)
- **PPK 解析**: 在 `format.go` 中实现了自定义头部解析逻辑。
- **签名验证**: 使用 `crypto/ed25519` 添加了 `VerifySignature` 逻辑。
- **解压缩**: 从 `sevenzip` 库切换到了标准的 `archive/zip` 库。
- **部署流程**:
    1. 解压 `potstack-base.zip`。
    2. 从基础包中加载公钥 (`potstack_release.pub`)。
    3. 解析 `install.yml` 清单。
    4. 对每个 PPK 文件：
        - 验证头部和签名。
        - 解压 Zip 内容。
        - 将组件 (`repo`, `loader`, `keeper`) 推送到内部 Git/Gitea。

### 3. 构建流程 (`build_base_pack.sh`)
- 如果密钥缺失，自动生成密钥对。
- 使用 `potpacker -k` 对包进行签名。
- 将公钥包含在最终的 `potstack-base.zip` 中。

## 验证
端到端测试确认：
- `potpacker` 正确生成了经过签名的 PPK 文件。
- `potStack` Loader 成功地：
    - 加载了公钥。
    - 验证了 PPK 签名。
    - 解压并部署了 Git 仓库。

## 下一步
- 确保 `potstack_release.key` 在生产环境中得到安全备份。
- 考虑在未来增加密钥轮换机制。
