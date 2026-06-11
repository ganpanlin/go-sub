// app.js — Vue root instance: shared state, auth, title bar

new Vue({
    el: '#app',
    data: {
        activeTab: 'sources',
        loading: false,
        refreshingSources: false,
        // Shared state
        sources: [],
        profiles: [],
        routingProfiles: [],
        ruleCatalog: [],
        versionInfo: {},
        // Auth
        authEnabled: false,
        needsSetup: false,
        loginVisible: false,
        loginForm: { username: 'admin', password: '' },
        setupVisible: false,
        setupForm: { username: 'admin', password: '', password2: '' },
        changePwdVisible: false,
        changePwdForm: { old_password: '', new_password: '', new_password2: '' }
    },
    methods: {
        // === Shared data fetching ===
        fetchAll: function () {
            this.$refs.sourceList.fetchSources();
            this.$refs.profileList.fetchProfiles();
            this.$refs.routingManager.fetchRouting();
        },
        onTabChange: function () { },

        // === Version ===
        fetchVersion: function () {
            axios.get('/api/version').then(function (r) { this.versionInfo = r.data || {}; }.bind(this)).catch(function () { });
        },

        // === Auth ===
        checkAuth: function () {
            var self = this;
            axios.get('/api/auth/status').then(function (r) {
                var d = r.data || {};
                self.authEnabled = !!d.enabled;
                self.needsSetup = !!d.needs_setup;
                if (self.needsSetup) {
                    self.setupVisible = true; self.loginVisible = false;
                } else if (self.authEnabled && !d.authenticated) {
                    self.loginVisible = true; self.setupVisible = false;
                } else {
                    self.loginVisible = false; self.setupVisible = false;
                    self.fetchAll();
                }
            }).catch(function () { self.loginVisible = true; });
        },
        login: function () {
            var self = this;
            axios.post('/api/auth/login', this.loginForm).then(function (r) {
                var d = r.data || {};
                if (d.needs_setup) { self.loginVisible = false; self.setupVisible = true; return; }
                setToken(d.token);
                self.$message.success('登录成功'); self.loginVisible = false; self.fetchAll();
            }).catch(function (e) { self.$message.error(apiErrorMessage(e, '登录失败')); });
        },
        doLogout: function () {
            var self = this;
            axios.post('/api/auth/logout').then(function () {
                clearToken();
                self.$message.success('已退出');
                self.loginForm.password = '';
                self.loginVisible = true;
            });
        },
        doSetup: function () {
            if (!this.setupForm.password || this.setupForm.password.length < 6) return this.$message.error('密码至少6位');
            if (this.setupForm.password !== this.setupForm.password2) return this.$message.error('两次密码不一致');
            var self = this;
            axios.post('/api/auth/setup', {
                username: this.setupForm.username || 'admin',
                password: this.setupForm.password
            }).then(function (r) {
                var d = r.data || {};
                setToken(d.token);
                self.$message.success('管理员账户已创建'); self.setupVisible = false; self.needsSetup = false; self.authEnabled = true; self.fetchAll();
            }).catch(function (e) { self.$message.error(apiErrorMessage(e, '创建失败')); });
        },
        doChangePassword: function () {
            if (!this.changePwdForm.new_password || this.changePwdForm.new_password.length < 6) return this.$message.error('新密码至少6位');
            if (this.changePwdForm.new_password !== this.changePwdForm.new_password2) return this.$message.error('两次密码不一致');
            var self = this;
            axios.post('/api/auth/change-password', {
                old_password: this.changePwdForm.old_password,
                new_password: this.changePwdForm.new_password
            }).then(function () {
                self.$message.success('密码已修改'); self.changePwdVisible = false;
                self.changePwdForm = { old_password: '', new_password: '', new_password2: '' };
            }).catch(function (e) { self.$message.error(apiErrorMessage(e, '修改失败')); });
        }
    },
    created: function () {
        this.fetchVersion();
        this.checkAuth();
        var self = this;
        axios.get('/api/routing/catalog').then(function (r) { self.ruleCatalog = r.data || []; }).catch(function () { });
    }
});
