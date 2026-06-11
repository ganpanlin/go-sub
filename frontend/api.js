// api.js — Axios configuration, JWT token management, shared utilities

(function () {
    var token = localStorage.getItem('pf_token');
    if (token) {
        axios.defaults.headers.common['Authorization'] = 'Bearer ' + token;
    }
})();

// Response interceptor: unwrap {code, msg, data} envelope
axios.interceptors.response.use(
    function (resp) {
        var body = resp.data;
        if (body && typeof body === 'object' &&
            Object.prototype.hasOwnProperty.call(body, 'code') &&
            Object.prototype.hasOwnProperty.call(body, 'data')) {
            if (body.code === 0) {
                resp.data = body.data;
                return resp;
            }
            var err = new Error(body.msg || '请求失败');
            err.response = resp;
            err.api = body;
            return Promise.reject(err);
        }
        return resp;
    },
    function (err) { return Promise.reject(err); }
);

function setToken(token) {
    if (token) {
        localStorage.setItem('pf_token', token);
        axios.defaults.headers.common['Authorization'] = 'Bearer ' + token;
    }
}

function clearToken() {
    localStorage.removeItem('pf_token');
    delete axios.defaults.headers.common['Authorization'];
}

function apiErrorMessage(err, fallback) {
    if (err && err.api && err.api.msg) return err.api.msg;
    if (err && err.response && err.response.data && err.response.data.msg) return err.response.data.msg;
    if (err && err.response && err.response.data && err.response.data.error) return err.response.data.error;
    return fallback || (err && err.message) || '请求失败';
}

// Shared utility functions used across components

function sourceTypeText(type) {
    if (type === 'local_uri') return '本地URI';
    if (type === 'local_yaml') return '本地YAML';
    return '远程URL';
}

function sourceLocationText(row) {
    if (row.type === 'local_uri') return 'local://uri';
    if (row.type === 'local_yaml') return 'local://yaml';
    return row.url || '';
}

function sourceKey(row) {
    return row.id || row.url;
}

function sourcePayload(row) {
    return { id: row.id };
}

function getStatusTagType(s) {
    if (s === 0) return 'info';
    if (s >= 200 && s < 300) return 'success';
    if (s >= 500) return 'danger';
    return 'warning';
}

function getStatusText(s) {
    return s === 0 ? '...' : s;
}
