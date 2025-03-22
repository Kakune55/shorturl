#!/bin/bash

# 需要root权限执行

# 增加文件描述符限制
echo "* soft nofile 1000000" >> /etc/security/limits.conf
echo "* hard nofile 1000000" >> /etc/security/limits.conf

# 优化TCP连接参数
cat >> /etc/sysctl.conf << EOF
# TCP优化
net.ipv4.tcp_fin_timeout = 30
net.ipv4.tcp_keepalive_time = 1200
net.ipv4.tcp_max_syn_backlog = 8192
net.ipv4.tcp_max_tw_buckets = 5000
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_slow_start_after_idle = 0
net.ipv4.ip_local_port_range = 1024 65535
net.core.somaxconn = 65535
net.core.netdev_max_backlog = 262144
EOF

# 应用配置
sysctl -p

# 设置进程最大可打开文件数
ulimit -n 1000000

echo "系统参数优化完成"
