document.addEventListener('DOMContentLoaded', function() {
    // 登录表单元素
    const loginForm = document.querySelector('.login-form');
    const registerForm = document.querySelector('.register-form');
    const showRegisterLink = document.getElementById('showRegister');
    const showLoginLink = document.getElementById('showLogin');
    const loginBtn = document.getElementById('loginBtn');
    const registerBtn = document.getElementById('registerBtn');
    const loginError = document.getElementById('error-message');
    const regError = document.getElementById('reg-error-message');
    
    // 切换表单显示
    showRegisterLink.addEventListener('click', function(e) {
        e.preventDefault();
        loginForm.style.display = 'none';
        registerForm.style.display = 'block';
    });
    
    showLoginLink.addEventListener('click', function(e) {
        e.preventDefault();
        registerForm.style.display = 'none';
        loginForm.style.display = 'block';
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
            if (data.error) {
                loginError.textContent = data.error;
                return;
            }
            
            // 保存令牌到Cookie
            document.cookie = `auth_token=${data.token}; path=/; max-age=${24*60*60}`; // 24小时
            
            // 重定向到仪表盘
            window.location.href = '/dashboard';
        })
        .catch(error => {
            console.error('登录错误:', error);
            loginError.textContent = '登录失败，请重试';
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
            regError.textContent = '请填写所有字段';
            return;
        }
        
        if (password !== passwordConfirm) {
            regError.textContent = '两次输入的密码不一致';
            return;
        }
        
        regError.textContent = '';
        
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
            if (data.error) {
                regError.textContent = data.error;
                return;
            }
            
            // 显示成功消息并切换到登录表单
            alert('注册成功，请登录');
            registerForm.style.display = 'none';
            loginForm.style.display = 'block';
        })
        .catch(error => {
            console.error('注册错误:', error);
            regError.textContent = '注册失败，请重试';
        });
    });
});
