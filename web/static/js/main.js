document.addEventListener('DOMContentLoaded', function() {
    const originalUrlInput = document.getElementById('originalUrl');
    const expirationSelect = document.getElementById('expiration');
    const generateBtn = document.getElementById('generateBtn');
    const resultDiv = document.getElementById('result');
    const shortUrlSpan = document.getElementById('shortUrl');
    const expiresAtSpan = document.getElementById('expiresAt');
    const copyBtn = document.getElementById('copyBtn');
    const notificationsContainer = document.getElementById('notifications');
    
    // 生成短链接
    generateBtn.addEventListener('click', function() {
        const originalUrl = originalUrlInput.value.trim();
        if (!originalUrl) {
            showNotification('请输入有效的URL', 'error');
            return;
        }
        
        // 检查URL格式
        if (!originalUrl.match(/^(http|https):\/\/.+/)) {
            showNotification('请输入包含http://或https://的完整URL', 'error');
            return;
        }
        
        // 显示加载状态
        generateBtn.disabled = true;
        generateBtn.innerHTML = '<i class="bx bx-loader-alt bx-spin"></i> 生成中...';
        
        // 请求API创建短链接
        fetch('/api/urls', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                original_url: originalUrl,
                expires_in: expirationSelect.value
            }),
        })
        .then(response => {
            if (!response.ok) {
                throw new Error('创建短链接失败');
            }
            return response.json();
        })
        .then(data => {
            // 恢复按钮状态
            generateBtn.disabled = false;
            generateBtn.textContent = '生成短链接';
            
            // 显示结果
            const fullShortUrl = window.location.origin + '/' + data.short_code;
            shortUrlSpan.textContent = fullShortUrl;
            shortUrlSpan.href = fullShortUrl;
            
            // 格式化过期时间
            const expiresAt = new Date(data.expires_at);
            expiresAtSpan.textContent = formatDateTime(expiresAt);
            
            // 显示结果区域
            resultDiv.style.display = 'block';
            
            // 显示成功通知
            showNotification('短链接创建成功！', 'success');
            
            // 滚动到结果区域
            resultDiv.scrollIntoView({ behavior: 'smooth', block: 'center' });
        })
        .catch(error => {
            console.error('错误:', error);
            generateBtn.disabled = false;
            generateBtn.textContent = '生成短链接';
            showNotification('生成短链接时出错，请重试', 'error');
        });
    });
    
    // 复制链接功能
    copyBtn.addEventListener('click', function() {
        const textToCopy = shortUrlSpan.textContent;
        
        // 使用现代的clipboard API
        if (navigator.clipboard) {
            navigator.clipboard.writeText(textToCopy)
                .then(() => {
                    copyBtn.innerHTML = '<i class="bx bx-check"></i> 已复制';
                    setTimeout(() => {
                        copyBtn.textContent = '复制';
                    }, 2000);
                    showNotification('链接已复制到剪贴板', 'success');
                })
                .catch(err => {
                    console.error('复制失败:', err);
                    showNotification('复制失败，请手动复制', 'error');
                });
        } else {
            // 后备方案
            const textarea = document.createElement('textarea');
            textarea.value = textToCopy;
            document.body.appendChild(textarea);
            textarea.select();
            
            try {
                document.execCommand('copy');
                copyBtn.innerHTML = '<i class="bx bx-check"></i> 已复制';
                setTimeout(() => {
                    copyBtn.textContent = '复制';
                }, 2000);
                showNotification('链接已复制到剪贴板', 'success');
            } catch (err) {
                console.error('复制失败:', err);
                showNotification('复制失败，请手动复制', 'error');
            }
            
            document.body.removeChild(textarea);
        }
    });
    
    // 通知系统
    function showNotification(message, type = 'info') {
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
});
