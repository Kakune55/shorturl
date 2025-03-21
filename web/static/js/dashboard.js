document.addEventListener('DOMContentLoaded', function() {
    // 初始化侧边栏切换
    initSidebar();
    
    // 初始化选项卡切换
    initTabs();
    
    // 初始化数据加载
    loadDashboardData();
    
    // 添加事件监听器
    addEventListeners();
    
    // 处理URL哈希变化
    handleHashChange();
    
    // 监听哈希变化
    window.addEventListener('hashchange', handleHashChange);
});

// 侧边栏功能
function initSidebar() {
    const sidebar = document.getElementById('sidebar');
    const menuToggle = document.getElementById('menu-toggle');
    const sidebarToggle = document.getElementById('sidebar-toggle');
    
    menuToggle.addEventListener('click', function() {
        sidebar.classList.add('show');
    });
    
    sidebarToggle.addEventListener('click', function() {
        sidebar.classList.remove('show');
    });
}

// 选项卡功能
function initTabs() {
    const tabs = document.querySelectorAll('.sidebar-nav a');
    
    tabs.forEach(tab => {
        tab.addEventListener('click', function(e) {
            e.preventDefault();
            const targetId = this.getAttribute('data-tab');
            
            // 更新当前URL哈希
            window.location.hash = targetId;
            
            // 激活对应的选项卡
            activateTab(targetId);
        });
    });
    
    // 退出登录按钮
    document.getElementById('logoutBtn').addEventListener('click', logout);
}

// 激活特定选项卡
function activateTab(tabId) {
    // 更新激活的导航项
    document.querySelectorAll('.sidebar-nav a').forEach(tab => {
        if (tab.getAttribute('data-tab') === tabId) {
            tab.classList.add('active');
        } else {
            tab.classList.remove('active');
        }
    });
    
    // 显示对应的内容
    document.querySelectorAll('.tab-content').forEach(content => {
        if (content.id === tabId) {
            content.classList.add('active');
        } else {
            content.classList.remove('active');
        }
    });
    
    // 加载对应选项卡的数据
    loadTabData(tabId);
}

// 处理URL哈希变化
function handleHashChange() {
    let hash = window.location.hash.substring(1);
    
    // 如果没有哈希或哈希不是有效的选项卡ID，则默认显示仪表盘
    if (!hash || !document.getElementById(hash)) {
        hash = 'dashboard';
        window.location.hash = hash;
    }
    
    activateTab(hash);
}

// 加载特定选项卡的数据
function loadTabData(tabId) {
    switch (tabId) {
        case 'dashboard':
            loadDashboardData();
            break;
        case 'links':
            loadUserLinks();
            break;
        case 'stats':
            // 不需要立即加载数据，用户需要先选择一个链接
            initStatsSearch();
            break;
        case 'admin':
            loadAdminData();
            break;
    }
}

// 加载仪表盘数据
function loadDashboardData() {
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    // 显示加载状态
    document.getElementById('total-links').textContent = '-';
    document.getElementById('total-visits').textContent = '-';
    document.getElementById('active-links').textContent = '-';
    document.getElementById('recent-links').innerHTML = '<tr><td colspan="5" class="text-center">加载中...</td></tr>';
    
    // 获取用户仪表盘数据
    fetch('/api/dashboard', {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('加载仪表盘数据失败');
        }
        return response.json();
    })
    .then(data => {
        // 更新统计卡片
        document.getElementById('total-links').textContent = data.total_links || 0;
        document.getElementById('total-visits').textContent = data.total_visits || 0;
        document.getElementById('active-links').textContent = data.active_links || 0;
        
        // 更新最近链接
        updateRecentLinks(data.recent_links || []);
        
        // 渲染访问趋势图表
        renderVisitsTrendChart(data.visits_trend || []);
    })
    .catch(error => {
        console.error('Error:', error);
        showNotification('加载仪表盘数据失败', 'error');
    });
}

// 更新最近链接表格
function updateRecentLinks(links) {
    const tbody = document.getElementById('recent-links');
    
    if (links.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="text-center">暂无链接数据</td></tr>';
        return;
    }
    
    tbody.innerHTML = '';
    links.forEach(link => {
        const row = document.createElement('tr');
        const shortUrl = window.location.origin + '/' + link.short_code;
        
        row.innerHTML = `
            <td class="url-code"><a href="${shortUrl}" target="_blank">${link.short_code}</a></td>
            <td class="url-original"><a href="${link.original_url}" target="_blank" title="${link.original_url}">${truncateString(link.original_url, 40)}</a></td>
            <td class="url-date">${formatDateTime(new Date(link.CreatedAt))}</td>
            <td class="url-visits">${link.visits}</td>
            <td class="actions-cell">
                <div class="btn-group">
                    <button class="btn btn-sm btn-outline-primary view-stats" data-code="${link.short_code}" title="查看统计">
                        <i class="bx bx-bar-chart-alt-2"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-danger delete-url" data-code="${link.short_code}" title="删除">
                        <i class="bx bx-trash"></i>
                    </button>
                </div>
            </td>
        `;
        
        tbody.appendChild(row);
    });
    
    // 添加事件监听器
    document.querySelectorAll('.view-stats').forEach(btn => {
        btn.addEventListener('click', function() {
            const code = this.getAttribute('data-code');
            window.location.hash = 'stats';
            selectUrlForStats(code);
        });
    });
    
    document.querySelectorAll('.delete-url').forEach(btn => {
        btn.addEventListener('click', function() {
            const code = this.getAttribute('data-code');
            confirmDelete(code);
        });
    });
}

// 渲染访问趋势图表
function renderVisitsTrendChart(trendData) {
    // 准备图表数据
    const ctx = document.getElementById('visitsTrendChart').getContext('2d');
    
    // 销毁已存在的图表 - 修复销毁方法检查
    if (window.visitsTrendChart && typeof window.visitsTrendChart.destroy === 'function') {
        window.visitsTrendChart.destroy();
    }
    
    // 如果没有数据，显示空图表
    if (!trendData || trendData.length === 0) {
        window.visitsTrendChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: '访问量',
                    data: [],
                    borderColor: 'rgba(52, 152, 219, 1)',
                    backgroundColor: 'rgba(52, 152, 219, 0.1)',
                    tension: 0.4,
                    fill: true
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    },
                    tooltip: {
                        mode: 'index',
                        intersect: false
                    }
                },
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
        return;
    }
    
    // 准备数据
    trendData.sort((a, b) => new Date(a.date) - new Date(b.date));
    const labels = trendData.map(item => formatDate(new Date(item.date)));
    const values = trendData.map(item => item.count);
    
    // 创建图表
    window.visitsTrendChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: '访问量',
                data: values,
                borderColor: 'rgba(52, 152, 219, 1)',
                backgroundColor: 'rgba(52, 152, 219, 0.1)',
                tension: 0.4,
                fill: true
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    mode: 'index',
                    intersect: false
                }
            },
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

// 加载用户链接
function loadUserLinks() {
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    const tableBody = document.getElementById('links-table-body');
    tableBody.innerHTML = '<tr><td colspan="6" class="text-center">正在加载...</td></tr>';
    
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
            tableBody.innerHTML = `
                <tr>
                    <td colspan="6">
                        <div class="empty-state">
                            <i class="bx bx-link-alt empty-icon"></i>
                            <div class="empty-message">您还没有创建任何短链接</div>
                            <a href="#create" class="btn btn-primary mt-3" data-tab="create">创建第一个短链接</a>
                        </div>
                    </td>
                </tr>
            `;
            return;
        }
        
        tableBody.innerHTML = '';
        urls.forEach(url => {
            const row = document.createElement('tr');
            
            // 格式化日期
            const createdAt = new Date(url.CreatedAt);
            const expiresAt = new Date(url.expires_at);
            
            // 构建短链接URL
            const shortUrl = window.location.origin + '/' + url.short_code;
            
            row.innerHTML = `
                <td class="url-code"><a href="${shortUrl}" target="_blank">${url.short_code}</a></td>
                <td class="url-original"><a href="${url.original_url}" target="_blank" title="${url.original_url}">${truncateString(url.original_url, 40)}</a></td>
                <td class="url-date">${formatDateTime(createdAt)}</td>
                <td class="url-date">${formatDateTime(expiresAt)}</td>
                <td class="url-visits">${url.visits}</td>
                <td class="actions-cell">
                    <div class="btn-group">
                        <button class="btn btn-sm btn-outline-primary copy-url" data-url="${shortUrl}" title="复制链接">
                            <i class="bx bx-copy"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-info view-stats" data-code="${url.short_code}" title="查看统计">
                            <i class="bx bx-bar-chart-alt-2"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-danger delete-url" data-code="${url.short_code}" title="删除">
                            <i class="bx bx-trash"></i>
                        </button>
                    </div>
                </td>
            `;
            
            tableBody.appendChild(row);
        });
        
        // 添加事件处理
        document.querySelectorAll('.copy-url').forEach(btn => {
            btn.addEventListener('click', function() {
                const url = this.getAttribute('data-url');
                copyToClipboard(url);
            });
        });
        
        document.querySelectorAll('.view-stats').forEach(btn => {
            btn.addEventListener('click', function() {
                const code = this.getAttribute('data-code');
                window.location.hash = 'stats';
                selectUrlForStats(code);
            });
        });
        
        document.querySelectorAll('.delete-url').forEach(btn => {
            btn.addEventListener('click', function() {
                const code = this.getAttribute('data-code');
                confirmDelete(code);
            });
        });
        
        // 初始化搜索功能
        initLinksSearch(urls);
    })
    .catch(error => {
        console.error('Error:', error);
        tableBody.innerHTML = '<tr><td colspan="6" class="text-center">加载失败，请刷新页面重试</td></tr>';
        showNotification('加载短链接列表失败', 'error');
    });
}

// 初始化链接搜索功能
function initLinksSearch(urls) {
    const searchInput = document.getElementById('links-search');
    const tableBody = document.getElementById('links-table-body');
    
    searchInput.addEventListener('input', function() {
        const query = this.value.toLowerCase();
        
        // 如果查询为空，显示所有链接
        if (!query) {
            document.querySelectorAll('#links-table-body tr').forEach(row => {
                row.style.display = '';
            });
            return;
        }
        
        // 过滤并只显示匹配的行
        document.querySelectorAll('#links-table-body tr').forEach(row => {
            const shortCode = row.querySelector('.url-code').textContent.toLowerCase();
            const originalUrl = row.querySelector('.url-original').textContent.toLowerCase();
            
            if (shortCode.includes(query) || originalUrl.includes(query)) {
                row.style.display = '';
            } else {
                row.style.display = 'none';
            }
        });
    });
    
    // 刷新按钮
    document.getElementById('refresh-links').addEventListener('click', loadUserLinks);
}

// 初始化统计搜索功能
function initStatsSearch() {
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    const searchInput = document.getElementById('stats-search');
    const optionsList = document.getElementById('stats-options');
    
    // 仅当输入字段存在时
    if (!searchInput) return;
    
    // 清空输入
    searchInput.value = '';
    
    // 获取所有链接用于搜索
    fetch('/api/urls', {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => response.json())
    .then(urls => {
        // 添加输入事件监听器
        searchInput.addEventListener('input', function() {
            const query = this.value.toLowerCase();
            
            if (!query) {
                optionsList.innerHTML = '';
                optionsList.style.display = 'none';
                return;
            }
            
            // 过滤匹配的链接
            const matches = urls.filter(url => 
                url.short_code.toLowerCase().includes(query) || 
                url.original_url.toLowerCase().includes(query)
            ).slice(0, 5);  // 最多显示5个结果
            
            // 显示搜索结果
            optionsList.innerHTML = '';
            
            if (matches.length === 0) {
                const li = document.createElement('div');
                li.className = 'search-item';
                li.textContent = '没有找到匹配的链接';
                optionsList.appendChild(li);
            } else {
                matches.forEach(url => {
                    const li = document.createElement('div');
                    li.className = 'search-item';
                    li.innerHTML = `
                        <strong>${url.short_code}</strong> - 
                        <span title="${url.original_url}">${truncateString(url.original_url, 30)}</span>
                    `;
                    li.setAttribute('data-code', url.short_code);
                    
                    li.addEventListener('click', function() {
                        const code = this.getAttribute('data-code');
                        selectUrlForStats(code);
                        searchInput.value = `${code} - ${truncateString(url.original_url, 30)}`;
                        optionsList.style.display = 'none';
                    });
                    
                    optionsList.appendChild(li);
                });
            }
            
            optionsList.style.display = 'block';
        });
        
        // 点击外部关闭下拉框
        document.addEventListener('click', function(e) {
            if (!searchInput.contains(e.target) && !optionsList.contains(e.target)) {
                optionsList.style.display = 'none';
            }
        });
        
        // 获取焦点时显示下拉框
        searchInput.addEventListener('focus', function() {
            if (this.value.trim() !== '') {
                optionsList.style.display = 'block';
            }
        });
    })
    .catch(error => {
        console.error('Error:', error);
        showNotification('加载链接数据失败', 'error');
    });
}

// 为统计选择URL
function selectUrlForStats(code) {
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    const statsContent = document.getElementById('stats-content');
    statsContent.innerHTML = '<div class="loading-spinner"><div class="spinner"></div></div>';
    
    fetch(`/api/urls/${code}/stats`, {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('加载统计数据失败');
        }
        return response.json();
    })
    .then(stats => {
        // 构建统计内容
        renderStatsContent(code, stats);
    })
    .catch(error => {
        console.error('Error:', error);
        statsContent.innerHTML = `
            <div class="empty-state">
                <i class="bx bx-error empty-icon"></i>
                <div class="empty-message">加载统计数据失败</div>
                <button class="btn btn-primary mt-3" onclick="selectUrlForStats('${code}')">重试</button>
            </div>
        `;
        showNotification('加载统计数据失败', 'error');
    });
}

// 渲染统计内容
function renderStatsContent(code, stats) {
    const shortUrl = window.location.origin + '/' + code;
    const statsContent = document.getElementById('stats-content');
    
    statsContent.innerHTML = `
        <div class="stats-container">
            <div class="card mb-4">
                <div class="card-header">
                    <h2>链接概览</h2>
                </div>
                <div class="card-body">
                    <div class="row">
                        <div class="col">
                            <p><strong>短链接:</strong> <a href="${shortUrl}" target="_blank">${shortUrl}</a></p>
                            <p><strong>总访问量:</strong> ${stats.total_visits || 0}</p>
                        </div>
                        <div class="col text-right">
                            <button id="export-stats" class="btn btn-primary" data-code="${code}">
                                <i class="bx bx-download"></i> 导出数据
                            </button>
                        </div>
                    </div>
                </div>
            </div>
            
            <div class="card mb-4">
                <div class="card-header">
                    <h2>访问趋势</h2>
                </div>
                <div class="chart-container" style="height: 300px;">
                    <canvas id="urlVisitsChart"></canvas>
                </div>
            </div>
            
            <div class="row">
                <div class="col">
                    <div class="card mb-4">
                        <div class="card-header">
                            <h2>主要来源网站</h2>
                        </div>
                        <div class="card-body">
                            <div id="referersChart"></div>
                        </div>
                    </div>
                </div>
                <div class="col">
                    <div class="card mb-4">
                        <div class="card-header">
                            <h2>设备与浏览器</h2>
                        </div>
                        <div class="card-body">
                            <div id="userAgentsChart"></div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `;
    
    // 渲染访问趋势图表
    renderUrlVisitsChart(stats.daily_visits || []);
    
    // 渲染来源网站列表
    renderReferersList(stats.top_referers || []);
    
    // 渲染用户代理列表
    renderUserAgentsList(stats.top_user_agents || []);
    
    // 添加导出事件
    document.getElementById('export-stats').addEventListener('click', function() {
        const code = this.getAttribute('data-code');
        exportStats(code);
    });
}

// 渲染URL访问趋势图表
function renderUrlVisitsChart(dailyVisits) {
    const ctx = document.getElementById('urlVisitsChart').getContext('2d');
    
    // 检查是否已存在图表实例并销毁
    if (window.urlVisitsChart && typeof window.urlVisitsChart.destroy === 'function') {
        window.urlVisitsChart.destroy();
    }
    
    // 准备数据
    dailyVisits.sort((a, b) => new Date(a.date) - new Date(b.date));
    const labels = dailyVisits.map(item => formatDate(new Date(item.date)));
    const data = dailyVisits.map(item => item.count);
    
    // 创建图表
    window.urlVisitsChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: '访问量',
                data: data,
                fill: true,
                backgroundColor: 'rgba(52, 152, 219, 0.1)',
                borderColor: 'rgba(52, 152, 219, 1)',
                tension: 0.4
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                tooltip: {
                    mode: 'index',
                    intersect: false
                }
            },
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
function renderReferersList(referers) {
    const container = document.getElementById('referersChart');
    
    if (!referers || referers.length === 0) {
        container.innerHTML = '<div class="empty-state"><p>暂无来源数据</p></div>';
        return;
    }
    
    let html = '<ul class="detail-list">';
    referers.forEach(referer => {
        const url = referer.url || '直接访问';
        html += `
            <li>
                <span class="detail-label" title="${url}">${truncateString(url, 30)}</span>
                <span class="detail-value">${referer.count}</span>
            </li>
        `;
    });
    html += '</ul>';
    
    container.innerHTML = html;
}

// 渲染用户代理列表
function renderUserAgentsList(userAgents) {
    const container = document.getElementById('userAgentsChart');
    
    if (!userAgents || userAgents.length === 0) {
        container.innerHTML = '<div class="empty-state"><p>暂无设备/浏览器数据</p></div>';
        return;
    }
    
    let html = '<ul class="detail-list">';
    userAgents.forEach(ua => {
        html += `
            <li>
                <span class="detail-label" title="${ua.name}">${truncateString(ua.name, 30)}</span>
                <span class="detail-value">${ua.count}</span>
            </li>
        `;
    });
    html += '</ul>';
    
    container.innerHTML = html;
}

// 加载管理员面板数据
function loadAdminData() {
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    // 显示加载状态
    document.getElementById('total-users').textContent = '-';
    document.getElementById('admin-total-links').textContent = '-';
    document.getElementById('expired-links').textContent = '-';
    document.getElementById('users-table').innerHTML = '<tr><td colspan="7" class="text-center">加载中...</td></tr>';
    
    // 获取管理员统计数据
    fetch('/api/admin/stats', {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('加载管理员数据失败');
        }
        return response.json();
    })
    .then(data => {
        // 更新统计卡片
        document.getElementById('total-users').textContent = data.total_users || 0;
        document.getElementById('admin-total-links').textContent = data.total_links || 0;
        document.getElementById('expired-links').textContent = data.expired_links || 0;
        
        // 确保显示总访问量
        if (document.getElementById('admin-total-visits')) {
            document.getElementById('admin-total-visits').textContent = data.total_visits || 0;
        }
        
        // 加载用户列表
        loadUsersList();
        
        // 添加清理按钮事件
        document.getElementById('cleanup-btn').addEventListener('click', cleanupExpiredUrls);
        
        // 添加备份按钮事件
        document.getElementById('backup-btn').addEventListener('click', exportSystemData);
    })
    .catch(error => {
        console.error('Error:', error);
        showNotification('加载管理员数据失败', 'error');
    });
}

// 加载用户列表
function loadUsersList() {
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    const usersTable = document.getElementById('users-table');
    
    fetch('/api/admin/users', {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('加载用户列表失败');
        }
        return response.json();
    })
    .then(users => {
        if (users.length === 0) {
            usersTable.innerHTML = '<tr><td colspan="7" class="text-center">暂无用户数据</td></tr>';
            return;
        }
        
        usersTable.innerHTML = '';
        users.forEach(user => {
            const row = document.createElement('tr');
            
            // 格式化日期
            const createdAt = new Date(user.CreatedAt);
            const lastLogin = new Date(user.last_login_at);
            
            row.innerHTML = `
                <td>${user.ID}</td>
                <td>${user.username}</td>
                <td>${user.email}</td>
                <td>${formatDateTime(createdAt)}</td>
                <td>${user.links_count || 0}</td>
                <td>${formatDateTime(lastLogin)}</td>
                <td class="actions-cell">
                    <div class="btn-group">
                        <button class="btn btn-sm btn-outline-primary" onclick="viewUserLinks(${user.ID})" title="查看链接">
                            <i class="bx bx-link"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-danger" onclick="resetUserPassword(${user.ID})" title="重置密码">
                            <i class="bx bx-reset"></i>
                        </button>
                    </div>
                </td>
            `;
            
            usersTable.appendChild(row);
        });
    })
    .catch(error => {
        console.error('Error:', error);
        usersTable.innerHTML = '<tr><td colspan="7" class="text-center">加载失败，请刷新页面重试</td></tr>';
        showNotification('加载用户列表失败', 'error');
    });
}

// 清理过期URL
function cleanupExpiredUrls() {
    if (!confirm('确定要清理所有过期的短链接？此操作不可恢复。')) {
        return;
    }
    
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    const cleanupBtn = document.getElementById('cleanup-btn');
    
    // 显示加载状态
    cleanupBtn.disabled = true;
    cleanupBtn.innerHTML = '<i class="bx bx-loader-alt bx-spin"></i> 清理中...';
    
    fetch('/api/urls/cleanup', {
        method: 'POST',
        headers: {
            'Authorization': `Bearer ${token}`
        }
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('清理过期链接失败');
        }
        return response.json();
    })
    .then(data => {
        cleanupBtn.disabled = false;
        cleanupBtn.innerHTML = '<i class="bx bx-trash"></i> 清理过期链接';
        
        showNotification(data.message || '清理成功', 'success');
        
        // 刷新管理员数据
        loadAdminData();
    })
    .catch(error => {
        console.error('Error:', error);
        cleanupBtn.disabled = false;
        cleanupBtn.innerHTML = '<i class="bx bx-trash"></i> 清理过期链接';
        
        showNotification('清理过期链接失败', 'error');
    });
}

// 导出系统数据
function exportSystemData() {
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    const backupBtn = document.getElementById('backup-btn');
    
    // 显示加载状态
    backupBtn.disabled = true;
    backupBtn.innerHTML = '<i class="bx bx-loader-alt bx-spin"></i> 导出中...';
    
    // 使用window.open直接下载文件
    window.open(`/api/admin/export?token=${token}`, '_blank');
    
    // 恢复按钮状态
    setTimeout(() => {
        backupBtn.disabled = false;
        backupBtn.innerHTML = '<i class="bx bx-download"></i> 导出系统数据';
        showNotification('系统数据导出已开始', 'success');
    }, 1000);
}

// 导出指定URL的统计数据
function exportStats(code) {
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    window.open(`/api/urls/${code}/export?token=${token}`, '_blank');
    showNotification('统计数据导出已开始', 'success');
}

// 确认删除对话框
function confirmDelete(code) {
    if (confirm(`确定要删除短链接 ${code} 吗？此操作不可恢复。`)) {
        deleteUrl(code);
    }
}

// 删除URL
function deleteUrl(code) {
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    fetch(`/api/urls/${code}`, {
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
        showNotification(data.message || '删除成功', 'success');
        
        // 刷新当前选项卡
        const currentTab = document.querySelector('.sidebar-nav a.active').getAttribute('data-tab');
        loadTabData(currentTab);
    })
    .catch(error => {
        console.error('Error:', error);
        showNotification('删除短链接失败', 'error');
    });
}

// 添加所有事件监听器
function addEventListeners() {
    // 创建短链接
    const createBtn = document.getElementById('create-btn');
    if (createBtn) {
        createBtn.addEventListener('click', createShortUrl);
    }
    
    // 注册其他事件...
}

// 创建短链接
function createShortUrl() {
    const token = getAuthToken();
    if (!token) {
        redirectToLogin();
        return;
    }
    
    const originalUrl = document.getElementById('create-url').value.trim();
    const expiration = document.getElementById('create-expiration').value;
    const createBtn = document.getElementById('create-btn');
    const resultDiv = document.getElementById('create-result');
    
    if (!originalUrl) {
        showNotification('请输入要缩短的URL', 'error');
        return;
    }
    
    // 验证URL格式
    if (!originalUrl.match(/^(http|https):\/\/.+/)) {
        showNotification('请输入包含http://或https://的完整URL', 'error');
        return;
    }
    
    // 显示加载状态
    createBtn.disabled = true;
    createBtn.innerHTML = '<i class="bx bx-loader-alt bx-spin"></i> 创建中...';
    
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
    .then(response => {
        if (!response.ok) {
            throw new Error('创建短链接失败');
        }
        return response.json();
    })
    .then(data => {
        // 恢复按钮状态
        createBtn.disabled = false;
        createBtn.innerHTML = '创建短链接';
        
        // 显示结果
        const shortUrl = window.location.origin + '/' + data.short_code;
        document.getElementById('new-url-text').textContent = shortUrl;
        document.getElementById('new-url-expires').textContent = `过期时间: ${formatDateTime(new Date(data.expires_at))}`;
        resultDiv.style.display = 'block';
        
        // 添加复制按钮事件
        document.getElementById('copy-new-url').addEventListener('click', function() {
            copyToClipboard(shortUrl);
        });
        
        // 显示成功通知
        showNotification('短链接创建成功', 'success');
        
        // 清空输入框
        document.getElementById('create-url').value = '';
        
        // 刷新仪表盘数据
        if (document.getElementById('dashboard').classList.contains('active')) {
            loadDashboardData();
        }
    })
    .catch(error => {
        console.error('Error:', error);
        createBtn.disabled = false;
        createBtn.innerHTML = '创建短链接';
        showNotification('创建短链接失败', 'error');
    });
}

// 复制到剪贴板
function copyToClipboard(text) {
    if (navigator.clipboard) {
        navigator.clipboard.writeText(text)
            .then(() => {
                showNotification('已复制到剪贴板', 'success');
            })
            .catch(err => {
                console.error('复制失败:', err);
                showNotification('复制失败，请手动复制', 'error');
            });
    } else {
        // 后备方案
        const textarea = document.createElement('textarea');
        textarea.value = text;
        document.body.appendChild(textarea);
        textarea.select();
        
        try {
            document.execCommand('copy');
            showNotification('已复制到剪贴板', 'success');
        } catch (err) {
            console.error('复制失败:', err);
            showNotification('复制失败，请手动复制', 'error');
        }
        
        document.body.removeChild(textarea);
    }
}

// 退出登录
function logout() {
    // 删除认证Cookie
    document.cookie = 'auth_token=; path=/; expires=Thu, 01 Jan 1970 00:00:01 GMT;';
    // 重定向到首页
    window.location.href = '/';
}

// 辅助函数 - 获取认证令牌
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

// 重定向到登录页面
function redirectToLogin() {
    window.location.href = '/admin';
}

// 截断字符串
function truncateString(str, maxLength) {
    if (!str) return '';
    if (str.length <= maxLength) return str;
    return str.substring(0, maxLength) + '...';
}

// 格式化日期时间
function formatDateTime(date) {
    return date.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    });
}

// 格式化日期
function formatDate(date) {
    return date.toLocaleDateString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit'
    });
}

// 通知系统
function showNotification(message, type = 'info') {
    const notificationsContainer = document.getElementById('notifications');
    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    
    let icon = 'bx-info-circle';
    let title = '提示';
    
    if (type === 'success') {
        icon = 'bx-check-circle';
        title = '成功';
    } else if (type === 'error') {
        icon = 'bx-error';
        title = '错误';
    }
    
    notification.innerHTML = `
        <div class="notification-header">
            <span class="notification-title"><i class="bx ${icon}"></i> ${title}</span>
            <button class="notification-close">&times;</button>
        </div>
        <div class="notification-message">${message}</div>
    `;
    
    notificationsContainer.appendChild(notification);
    
    // 添加关闭按钮事件
    const closeBtn = notification.querySelector('.notification-close');
    closeBtn.addEventListener('click', function() {
        notification.classList.add('hide');
        setTimeout(() => {
            notificationsContainer.removeChild(notification);
        }, 300);
    });
    
    // 自动关闭通知
    setTimeout(() => {
        if (notification.parentNode) {
            notification.classList.add('hide');
            setTimeout(() => {
                if (notification.parentNode) {
                    notificationsContainer.removeChild(notification);
                }
            }, 300);
        }
    }, 5000);
}
