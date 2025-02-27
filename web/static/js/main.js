document.addEventListener('DOMContentLoaded', function() {
    const originalUrlInput = document.getElementById('originalUrl');
    const expirationSelect = document.getElementById('expiration');
    const generateBtn = document.getElementById('generateBtn');
    const resultDiv = document.getElementById('result');
    const shortUrlSpan = document.getElementById('shortUrl');
    const expiresAtSpan = document.getElementById('expiresAt');
    const copyBtn = document.getElementById('copyBtn');
    
    // 生成短链接
    generateBtn.addEventListener('click', function() {
        const originalUrl = originalUrlInput.value.trim();
        if (!originalUrl) {
            alert('请输入有效的URL');
            return;
        }
        
        // 检查URL格式
        if (!originalUrl.match(/^(http|https):\/\/.+/)) {
            alert('请输入包含http://或https://的完整URL');
            return;
        }
        
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
            // 显示结果
            shortUrlSpan.textContent = data.short_url;
            shortUrlSpan.href = data.short_url;
            
            // 格式化过期时间
            const expiresAt = new Date(data.expires_at);
            expiresAtSpan.textContent = expiresAt.toLocaleString();
            
            // 显示结果区域
            resultDiv.style.display = 'block';
        })
        .catch(error => {
            console.error('错误:', error);
            alert('生成短链接时出错，请重试');
        });
    });
    
    // 复制链接功能
    copyBtn.addEventListener('click', function() {
        const textToCopy = shortUrlSpan.textContent;
        
        // 使用现代的clipboard API
        if (navigator.clipboard) {
            navigator.clipboard.writeText(textToCopy)
                .then(() => {
                    copyBtn.textContent = '已复制';
                    setTimeout(() => {
                        copyBtn.textContent = '复制';
                    }, 2000);
                })
                .catch(err => {
                    console.error('复制失败:', err);
                    alert('复制失败，请手动复制');
                });
        } else {
            // 后备方案
            const textarea = document.createElement('textarea');
            textarea.value = textToCopy;
            document.body.appendChild(textarea);
            textarea.select();
            
            try {
                document.execCommand('copy');
                copyBtn.textContent = '已复制';
                setTimeout(() => {
                    copyBtn.textContent = '复制';
                }, 2000);
            } catch (err) {
                console.error('复制失败:', err);
                alert('复制失败，请手动复制');
            }
            
            document.body.removeChild(textarea);
        }
    });
});
