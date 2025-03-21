document.addEventListener('DOMContentLoaded', function() {
    // 登录表单元素
    const loginCard = document.getElementById('login-card');
    const registerCard = document.getElementById('register-card');
    const showRegisterLink = document.getElementById('showRegister');
    const showLoginLink = document.getElementById('showLogin');
    const loginBtn = document.getElementById('loginBtn');
    const registerBtn = document.getElementById('registerBtn');
    const loginError = document.getElementById('error-message');
    const regError = document.getElementById('reg-error-message');
    const notificationsContainer = document.getElementById('notifications');
    
    // 切换表单显示
    showRegisterLink.addEventListener('click', function(e) {
        e.preventDefault();
        loginCard.style.display = 'none';
        registerCard.style.display = 'block';
        // 清除表单错误
        loginError.textContent = '';
        regError.textContent = '';
    });
    
    showLoginLink.addEventListener('click', function(e) {
        e.preventDefault();
        registerCard.style.display = 'none';
        loginCard.style.display = 'block';
        // 清除表单错误
        loginError.textContent = '';
        regError.textContent = '';
    });
    
    // 登录处理
    loginBtn.addEventListener('click', function() {
        const username = document.getElementById('username').value.trim();
        const password = document.getElementById('password').value.trim();
        
        if (!username || !password) {
            loginError.textContent = '请输入用户名和密码';
            return;
        }
        
        loginError.textContent = '';
        
        // 显示加载状态
        loginBtn.disabled = true;
        loginBtn.innerHTML = '<i class="bx bx-loader-alt bx-spin"></i> 登录中...';
        
        fetch('/api/auth/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                username: username,
                password: password
            }),
        })
        .then(response => response.json())
        .then(data => {
            // 恢复按钮状态
            loginBtn.disabled = false;
            loginBtn.textContent = '登录';
            
            if (data.error) {
                loginError.textContent = data.error;
                showNotification(data.error, 'error');
                return;
            }
            
            // 保存令牌到Cookie
            document.cookie = `auth_token=${data.token}; path=/; max-age=${24*60*60}`; // 24小时
            
            // 显示成功通知
            showNotification('登录成功，正在跳转...', 'success');
            
            // 重定向到仪表盘
            setTimeout(() => {
                window.location.href = '/dashboard';
            }, 1000);
        })
        .catch(error => {
            console.error('登录错误:', error);
            loginBtn.disabled = false;
            loginBtn.textContent = '登录';
            loginError.textContent = '登录失败，请重试';
            showNotification('登录失败，请重试', 'error');
        });
    });
    
    // 注册处理
    registerBtn.addEventListener('click', function() {
        const username = document.getElementById('reg-username').value.trim();
        const email = document.getElementById('reg-email').value.trim();
        const password = document.getElementById('reg-password').value.trim();
        const passwordConfirm = document.getElementById('reg-password-confirm').value.trim();
        
        // 简单验证
        if (!username || !email || !password) {
            regError.textContent = '请填写所有必填字段';
            return;
        }
        
        if (username.length < 3) {
            regError.textContent = '用户名长度至少需要3个字符';
            return;
        }
        
        if (password.length < 6) {
            regError.textContent = '密码长度至少需要6个字符';
            return;
        }
        
        if (password !== passwordConfirm) {
            regError.textContent = '两次输入的密码不一致';
            return;
        }
        
        if (!validateEmail(email)) {
            regError.textContent = '请输入有效的电子邮箱地址';
            return;
        }
        
        regError.textContent = '';
        
        // 显示加载状态
        registerBtn.disabled = true;
        registerBtn.innerHTML = '<i class="bx bx-loader-alt bx-spin"></i> 注册中...';
        
        fetch('/api/auth/register', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                username: username,
                email: email,
                password: password
            }),
        })
        .then(response => response.json())
        .then(data => {
            // 恢复按钮状态
            registerBtn.disabled = false;
            registerBtn.textContent = '注册';
            
            if (data.error) {
                regError.textContent = data.error;
                showNotification(data.error, 'error');
                return;
            }
            
            // 显示成功消息并切换到登录表单
            showNotification('注册成功，请登录', 'success');
            setTimeout(() => {
                registerCard.style.display = 'none';
                loginCard.style.display = 'block';
                
                // 自动填充用户名
                document.getElementById('username').value = username;
                document.getElementById('password').focus();
            }, 1000);
        })
        .catch(error => {
            console.error('注册错误:', error);
            registerBtn.disabled = false;
            registerBtn.textContent = '注册';
            regError.textContent = '注册失败，请重试';
            showNotification('注册失败，请重试', 'error');
        });
    });
    
    // 回车键触发登录/注册
    document.getElementById('password').addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            loginBtn.click();
        }
    });
    
    document.getElementById('reg-password-confirm').addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            registerBtn.click();
        }
    });
    
    // 辅助函数 - 验证邮箱格式
    function validateEmail(email) {
        const re = /^(([^<>()\[\]\\.,;:\s@"]+(\.[^<>()\[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;
        return re.test(String(email).toLowerCase());
    }
    
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
});
