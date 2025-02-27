# ShortURL 系统

一个高性能的短链接系统，支持多种部署模式和可选的Redis缓存加速。

## 功能特点

- 支持Lite模式(SQLite)和标准模式(PostgreSQL)
- 可选的Redis缓存加速
- RESTful API管理接口
- Web管理界面，带有用户认证
- 访问统计与分析
- 高性能302重定向

## 技术栈

- **Web框架**: Gin (最新版本)
- **ORM**: GORM (最新版本) 
- **数据库**: SQLite / PostgreSQL
- **缓存**: Redis (可选)
- **配置管理**: Viper
- **认证**: JWT

## 安装与配置

### 前提条件

- Go 1.19+
- 对于标准模式: PostgreSQL 
- 对于缓存加速: Redis

### 快速开始

1. 克隆仓库

```bash
git clone https://github.com/yourusername/shorturl.git
cd shorturl
```

2. 安装依赖

```bash
make deps
```

3. 配置

编辑 `config/config.yaml` 文件，根据您的需求调整配置:

```yaml
server:
  port: 8080
  host: 0.0.0.0
  base_url: "http://localhost:8080" # 短链接的前缀URL

database:
  # 选择 sqlite 或 postgres
  type: sqlite
  # SQLite配置
  path: "./data/shorturl.db"
  # PostgreSQL配置
  host: localhost
  port: 5432
  user: postgres
  password: postgres
  dbname: shorturl
  sslmode: disable

redis:
  enabled: false # 设置为 true 启用Redis缓存
  host: localhost
  port: 6379
  password: ""
  db: 0
  max_memory: "100MB" # Redis内存限制
```

4. 运行

```bash
make run
```

或者直接编译后运行:

```bash
make build
./shorturl
```

### Docker部署

1. 构建Docker镜像

```bash
make docker
```

2. 运行容器

```bash
make docker-run
```

## API文档

### 公共API（无需认证）

#### 创建短链接

```
POST /api/urls
```

请求体:
```json
{
  "original_url": "https://example.com/very/long/url/that/needs/to/be/shortened",
  "expires_in": "24h" // 可选, 支持格式: "24h", "7d", "30d", "365d"
}
```

响应:
```json
{
  "short_code": "abc123",
  "original_url": "https://example.com/very/long/url/that/needs/to/be/shortened",
  "short_url": "http://localhost:8080/abc123",
  "expires_at": "2023-12-31T23:59:59Z"
}
```

#### 用户注册

```
POST /api/auth/register
```

请求体:
```json
{
  "username": "newuser",
  "email": "user@example.com",
  "password": "secure_password"
}
```

#### 用户登录

```
POST /api/auth/login
```

请求体:
```json
{
  "username": "newuser",
  "password": "secure_password"
}
```

响应:
```json
{
  "message": "登录成功",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 认证API（需要认证）

需要在请求头中添加 `Authorization: Bearer <token>`。

#### 获取用户短链接

```
GET /api/urls
```

#### 删除短链接

```
DELETE /api/urls/:code
```

#### 获取短链接统计

```
GET /api/urls/:code/stats
```

## 默认账户

首次启动时，系统会自动创建一个管理员账户:

- 用户名: `admin`
- 密码: `admin123`

**强烈建议**在生产环境中立即修改默认密码。

## 性能优化

短链接系统为高性能设计，特别是在重定向路径上:

1. **异步统计记录**: 访问统计使用goroutines异步处理，不影响重定向速度
2. **Redis缓存**: 启用Redis后，热门链接会缓存在内存中，显著减少数据库查询
3. **302临时重定向**: 使用302重定向允许未来更改目标URL

## 贡献

欢迎贡献代码、报告问题或提出功能请求。

## 许可证

MIT
