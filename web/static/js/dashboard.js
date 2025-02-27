document.addEventListener('DOMContentLoaded', function() {
    // 选项卡切换
    const tabs = document.querySelectorAll('.sidebar li');
    const tabContents = document.querySelectorAll('.tab-content');
    
    tabs.forEach(tab => {
        tab.addEventListener('click', function() {
            const tabId = this.getAttribute('data-tab');
            
            // 更新激活的选项卡
            tabs.forEach(t => t.classList.remove('active'));
            this.classList.add('active');
            
            // 显示对应的内容
            tabContents.forEach(content => {
                content.classList.remove('active');
                if (content.id === tabId) {
                    content.classList.add('active');
                }
            });
            
            // 如果点击的是"我的链接"选项卡，加载数据
            if (tabId === 'links') {
                loadUserLinks();
            } else if (tabId === 'stats') {
                loadUrlSelector();
            }
        });
    });
    
    // 退出登录按钮
    document.getElementById('logoutBtn').addEventListener('click', function() {
        // 删除认证Cookie
        document.cookie = 'auth_token=; path=/; expires=Thu, 01 Jan 1970 00:00:01 GMT;';
        // 重定向到首页
        window.location.href = '/';
    });
    
    // 创建短链接按钮
    document.getElementById('create-btn').addEventListener('click', createShortUrl);
    
    // 初始加载用户链接
    loadUserLinks();
    
    // 初始化统计链接选择器
    document.getElementById('stats-url').addEventListener('change', function() {
        const shortCode = this.value;
        if (shortCode) {
            loadUrlStats(shortCode);
        }
    });
});

// 获取认证令牌
function getAuthToken() {
    const cookies = document.cookie.split(';');
    for (let cookie of cookies) {
        const [name, value] = cookie.trim().split('=');
        if (name === 'auth_token') {
            return value;
        }
    }
    return null;
}

// 加载用户的链接
function loadUserLinks() {
    const token = getAuthToken();
    if (!token) {
        window.location.href = '/admin';
        return;
    }
    
    const tableBody = document.getElementById('links-table-body');
    tableBody.innerHTML = '<tr><td colspan="6">正在加载...</td></tr>';
    
    fetch('/api/urls', {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('加载链接失败');
        }
        return response.json();
    })
    .then(urls => {
        if (urls.length === 0) {
            tableBody.innerHTML = '<tr><td colspan="6">暂无短链接</td></tr>';
            return;
        }
        
        tableBody.innerHTML = '';
        urls.forEach(url => {
            const row = document.createElement('tr');
            
            // 格式化日期
            const createdAt = new Date(url.created_at).toLocaleString();
            const expiresAt = new Date(url.expires_at).toLocaleString();
            
            // 构建短链接URL
            const shortUrl = window.location.origin + '/' + url.short_code;
            
            row.innerHTML = `
                <td><a href="${shortUrl}" target="_blank">${url.short_code}</a></td>
                <td><a href="${url.original_url}" target="_blank">${truncateString(url.original_url, 50)}</a></td>
                <td>${createdAt}</td>
                <td>${expiresAt}</td>
                <td>${url.visits}</td>
                <td>
                    <button class="stats-btn" data-code="${url.short_code}">统计</button>
                    <button class="delete-btn" data-code="${url.short_code}">删除</button>
                </td>
            `;
            
            tableBody.appendChild(row);
        });
        
        // 添加统计按钮事件
        document.querySelectorAll('.stats-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                const code = this.getAttribute('data-code');
                
                // 切换到统计选项卡
                document.querySelector('[data-tab="stats"]').click();
                
                // 选择对应的URL
                document.getElementById('stats-url').value = code;
                
                // 加载统计数据
                loadUrlStats(code);
            });
        });
        
        // 添加删除按钮事件
        document.querySelectorAll('.delete-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                const code = this.getAttribute('data-code');
                if (confirm('确定要删除此短链接吗？此操作不可恢复。')) {
                    deleteUrl(code);
                }
            });
        });
    })
    .catch(error => {
        console.error('Error:', error);
        tableBody.innerHTML = '<tr><td colspan="6">加载失败，请刷新页面重试</td></tr>';
    });
}

// 加载统计选择器
function loadUrlSelector() {
    const token = getAuthToken();
    if (!token) {
        window.location.href = '/admin';
        return;
    }
    
    const selector = document.getElementById('stats-url');
    
    fetch('/api/urls', {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => response.json())
    .then(urls => {
        // 保存当前选择的值
        const currentValue = selector.value;
        
        // 清空选择器（保留第一个选项）
        while (selector.options.length > 1) {
            selector.remove(1);
        }
        
        // 添加新选项
        urls.forEach(url => {
            const option = document.createElement('option');
            option.value = url.short_code;
            option.textContent = `${url.short_code} (${truncateString(url.original_url, 30)})`;
            selector.appendChild(option);
        });
        
        // 如果之前有选择的值，尝试恢复
        if (currentValue) {
            selector.value = currentValue;
        }
    })
    .catch(error => {
        console.error('Error:', error);
    });
}

// 加载URL统计
function loadUrlStats(shortCode) {
    const token = getAuthToken();
    if (!token) {
        window.location.href = '/admin';
        return;
    }
    
    const statsContent = document.getElementById('stats-content');
    statsContent.innerHTML = '<div class="loading">加载统计数据...</div>';
    
    fetch(`/api/urls/${shortCode}/stats`, {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => response.json())
    .then(stats => {
        // 构建统计内容HTML
        let html = `
            <div class="stats-summary">
                <div class="stat-card">
                    <h3>总访问量</h3>
                    <div class="stat-value">${stats.total_visits}</div>
                </div>
            </div>
            
            <div class="stats-charts">
                <div class="stat-card">
                    <h3>每日访问趋势</h3>
                    <canvas id="dailyVisitsChart"></canvas>
                </div>
                
                <div class="stat-card">
                    <h3>主要来源网站</h3>
                    <div id="topReferers">
                        ${renderTopReferers(stats.top_referers)}
                    </div>
                </div>
                
                <div class="stat-card">
                    <h3>主要浏览器/设备</h3>
                    <div id="topUserAgents">
                        ${renderTopUserAgents(stats.top_user_agents)}
                    </div>
                </div>
            </div>
        `;
        
        statsContent.innerHTML = html;
        
        // 添加导出按钮
        addExportButton(shortCode);
        
        // 渲染每日访问趋势图表
        renderDailyVisitsChart(stats.daily_visits);
    })
    .catch(error => {
        console.error('Error:', error);
        statsContent.innerHTML = '<div class="error-message">加载统计数据失败</div>';
    });
}

// 渲染每日访问趋势图表
function renderDailyVisitsChart(dailyVisits) {
    if (!dailyVisits || dailyVisits.length === 0) {
        return;
    }
    
    const ctx = document.getElementById('dailyVisitsChart').getContext('2d');
    
    // 排序数据按日期
    dailyVisits.sort((a, b) => new Date(a.date) - new Date(b.date));
    
    const labels = dailyVisits.map(item => item.date);
    const data = dailyVisits.map(item => item.count);
    
    new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: '访问量',
                data: data,
                fill: false,
                borderColor: '#3498db',
                tension: 0.1
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        precision: 0
                    }
                }
            }
        }
    });
}

// 渲染来源网站列表
function renderTopReferers(referers) {
    if (!referers || referers.length === 0) {
        return '<p>无来源数据</p>';
    }
    
    let html = '<ul class="stats-list">';
    referers.forEach(referer => {
        const url = referer.url || '直接访问';
        html += `<li><span class="stats-label">${truncateString(url, 30)}</span> <span class="stats-value">${referer.count}</span></li>`;
    });
    html += '</ul>';
    
    return html;
}

// 渲染用户代理列表
function renderTopUserAgents(userAgents) {
    if (!userAgents || userAgents.length === 0) {
        return '<p>无浏览器/设备数据</p>';
    }
    
    let html = '<ul class="stats-list">';
    userAgents.forEach(ua => {
        html += `<li><span class="stats-label">${truncateString(ua.name, 30)}</span> <span class="stats-value">${ua.count}</span></li>`;
    });
    html += '</ul>';
    
    return html;
}

// 添加导出统计数据功能
function addExportButton(shortCode) {
    const statsContent = document.getElementById('stats-content');
    const exportDiv = document.createElement('div');
    exportDiv.className = 'export-container';
    exportDiv.innerHTML = `
        <button id="export-btn" class="export-btn">导出CSV统计数据</button>
    `;
    
    statsContent.prepend(exportDiv);
    
    document.getElementById('export-btn').addEventListener('click', function() {
        const token = getAuthToken();
        if (!token) {
            window.location.href = '/admin';
            return;
        }
        
        // 使用window.open直接下载文件
        window.open(`/api/urls/${shortCode}/export?token=${token}`, '_blank');
    });
}

// 创建短链接
function createShortUrl() {
    const token = getAuthToken();
    if (!token) {
        window.location.href = '/admin';
        return;
    }
    
    const originalUrl = document.getElementById('create-url').value.trim();
    const expiration = document.getElementById('create-expiration').value;
    const resultDiv = document.getElementById('create-result');
    
    if (!originalUrl) {
        resultDiv.className = 'create-result error';
        resultDiv.textContent = '请输入要缩短的URL';
        return;
    }
    
    // 验证URL格式
    if (!originalUrl.match(/^(http|https):\/\/.+/)) {
        resultDiv.className = 'create-result error';
        resultDiv.textContent = '请输入包含http://或https://的完整URL';
        return;
    }
    
    resultDiv.className = 'create-result';
    resultDiv.textContent = '正在创建...';
    
    fetch('/api/urls', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
            original_url: originalUrl,
            expires_in: expiration
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            throw new Error(data.error);
        }
        
        resultDiv.className = 'create-result success';
        resultDiv.innerHTML = `
            <p>短链接已创建成功：</p>
            <p><a href="${data.short_url}" target="_blank">${data.short_url}</a></p>
            <button id="copy-new-url">复制</button>
        `;
        
        // 添加复制功能
        document.getElementById('copy-new-url').addEventListener('click', function() {
            navigator.clipboard.writeText(data.short_url)
                .then(() => {
                    this.textContent = '已复制';
                    setTimeout(() => {
                        this.textContent = '复制';
                    }, 2000);
                })
                .catch(err => {
                    console.error('复制失败:', err);
                });
        });
        
        // 重新加载链接列表
        loadUserLinks();
        // 更新统计选择器
        loadUrlSelector();
    })
    .catch(error => {
        console.error('Error:', error);
        resultDiv.className = 'create-result error';
        resultDiv.textContent = '创建短链接失败: ' + error.message;
    });
}

// 删除URL
function deleteUrl(shortCode) {
    const token = getAuthToken();
    if (!token) {
        window.location.href = '/admin';
        return;
    }
    
    fetch(`/api/urls/${shortCode}`, {
        method: 'DELETE',
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('删除失败');
        }
        return response.json();
    })
    .then(data => {
        // 重新加载链接列表
        loadUserLinks();
        // 更新统计选择器
        loadUrlSelector();
    })
    .catch(error => {
        console.error('Error:', error);
        alert('删除短链接失败，请重试');
    });
}

// 工具函数：截断字符串
function truncateString(str, maxLength) {
    if (!str) return '';
    if (str.length <= maxLength) return;
    return str.substring(0, maxLength) + '...';
}
