-- chat-indicator-statistics（chat-stats profile，可选服务）的目标库。
-- 汇总表/配置表由其启动时 GORM AutoMigrate 自建，无需扩展。
-- 注意：initdb.d 仅在数据卷为空的首次启动执行；已有部署需手动建库一次：
--   docker compose exec postgres psql -U postgres -c "CREATE DATABASE chat_summary;"
CREATE DATABASE chat_summary;
