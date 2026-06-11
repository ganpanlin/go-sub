// source-list.js — 订阅源管理 tab：表格、添加/编辑/查看对话框、批量操作

Vue.component('source-list', {
    data: function () {
        return {
            selectedSources: [],
            batchTesting: false,
            testingStatus: {},
            // Add source dialog
            addSourceVisible: false,
            addSourceForm: { name: '', type: 'remote_url', url: '', content: '', ua: 'clash', enabled: true },
            // Edit source dialog
            sourceDialogVisible: false,
            sourceDialogForm: { id: '', name: '', type: 'remote_url', url: '', ua: 'clash', enabled: true },
            // Source data viewer
            sourceDataVisible: false,
            sourceDataLoading: false,
            sourceData: {}
        };
    },
    computed: {
        sources: function () { return this.$root.sources; },
        loading: function () { return this.$root.loading; }
    },
    methods: {
        // --- Data ---
        fetchSources: function () {
            var self = this;
            if (this._fetchTimer) clearTimeout(this._fetchTimer);
            this._fetchTimer = setTimeout(function () {
                if (self._fetchCancel) self._fetchCancel.cancel();
                var CancelToken = axios.CancelToken;
                var src = CancelToken.source();
                self._fetchCancel = src;
                self.$root.loading = true;
                axios.get('/api/sources', { cancelToken: src.token }).then(function (r) {
                    self.$root.sources = r.data || [];
                }).catch(function (e) {
                    if (!axios.isCancel(e)) console.error('fetchSources failed:', e);
                }).finally(function () {
                    self.$root.loading = false;
                    self._fetchCancel = null;
                });
            }, 500);
        },
        refreshSourceCache: function () {
            var self = this;
            this.$confirm('将强制重新获取所有已启用订阅源，并覆盖源缓存。继续？', '全量刷新缓存', { type: 'warning' }).then(function () {
                self.$root.refreshingSources = true;
                axios.post('/api/sources/refresh').then(function (resp) {
                    var r = resp.data || {};
                    self.$message.success('刷新完成：成功 ' + (r.success || 0) + '/' + (r.total || 0) + '，失败 ' + (r.failed || 0) + '，跳过 ' + (r.skipped || 0));
                    self.fetchSources();
                }).catch(function (e) {
                    self.$message.error('全量刷新失败: ' + apiErrorMessage(e));
                }).finally(function () {
                    self.$root.refreshingSources = false;
                });
            }).catch(function () { });
        },

        // --- Add source ---
        openAddSourceDialog: function () {
            this.addSourceForm = { name: '', type: 'remote_url', url: '', content: '', ua: 'clash', enabled: true };
            this.addSourceVisible = true;
        },
        doAddSource: function () {
            var f = this.addSourceForm;
            var self = this;
            if (f.type === 'remote_url' && !f.url) return this.$message.warning('URL 不能为空');
            if (f.type !== 'remote_url' && !f.content) return this.$message.warning('本地内容不能为空');
            if (!f.name) return this.$message.warning('名称不能为空');
            axios.post('/api/sources', {
                type: f.type, url: f.type === 'remote_url' ? f.url : '',
                content: f.type === 'remote_url' ? '' : f.content,
                name: f.name, ua: f.ua, enabled: f.enabled
            }).then(function () {
                self.$message.success('添加成功'); self.addSourceVisible = false; self.fetchSources();
            }).catch(function (e) { self.$message.error('添加失败: ' + apiErrorMessage(e)); });
        },

        // --- Edit source ---
        openSourceDialog: function (row) {
            this.sourceDialogForm = {
                id: row.id || '',
                name: (row.name && row.name !== row.url) ? row.name : '',
                type: row.type || 'remote_url',
                url: row.url,
                ua: row.ua || 'clash',
                enabled: row.enabled !== false
            };
            this.sourceDialogVisible = true;
        },
        saveSourceDialog: function () {
            var f = this.sourceDialogForm;
            var self = this;
            if (!f.name) return this.$message.warning('名称不能为空');
            axios.put('/api/sources', { id: f.id, url: f.url, name: f.name, ua: f.ua, enabled: f.enabled }).then(function () {
                self.$message.success('保存成功'); self.sourceDialogVisible = false; self.fetchSources();
            }).catch(function (e) { self.$message.error('保存失败: ' + apiErrorMessage(e)); });
        },

        // --- Delete ---
        deleteSource: function (row) {
            var self = this;
            this.$confirm('确认删除？', '提示', { type: 'warning' }).then(function () {
                axios.delete('/api/sources', { data: sourcePayload(row) }).then(function () {
                    self.$message.success('已删除'); self.fetchSources();
                });
            }).catch(function () { });
        },

        // --- Test ---
        testSource: function (row) {
            var key = sourceKey(row);
            var self = this;
            this.$set(this.testingStatus, key, true);
            axios.post('/api/sources/test', sourcePayload(row)).then(function (resp) {
                var r = resp.data || {};
                if (typeof r !== 'object') {
                    self.$message.warning('测试请求已提交'); setTimeout(function () { self.fetchSources(); }, 1500); return;
                }
                if (r.error) {
                    self.$message.error((r.status || 0) + ' ' + (r.latency || 0) + 'ms ' + r.error);
                } else {
                    self.$message.success(r.status + ' ' + r.latency + 'ms' + (r.is_cached ? ' 缓存' : ''));
                }
                self.fetchSources();
            }).catch(function (e) { self.$message.error('测试失败: ' + e); })
                .finally(function () { self.$set(self.testingStatus, key, false); });
        },
        toggleSourceEnabled: function (row) {
            var self = this;
            axios.put('/api/sources', Object.assign(sourcePayload(row), {
                name: row.name, ua: row.ua || '', enabled: row.enabled !== false
            })).then(function () {
                self.$message.success(row.enabled ? '已启用' : '已停用');
                self.fetchSources();
            }).catch(function (e) {
                row.enabled = !row.enabled;
                self.$message.error('保存失败: ' + apiErrorMessage(e));
            });
        },

        // --- Batch ---
        onSourceSelect: function (rows) { this.selectedSources = rows; },
        batchTest: function () {
            if (!this.selectedSources.length) return;
            var self = this;
            this.batchTesting = true;
            var done = 0, total = this.selectedSources.length;
            this.selectedSources.forEach(function (s) {
                var key = sourceKey(s);
                self.$set(self.testingStatus, key, true);
                axios.post('/api/sources/test', sourcePayload(s)).then(function (resp) {
                    var r = resp.data || {};
                    s.status = r.status || 0;
                    s.latency = r.latency || 0;
                    s.is_cached = r.is_cached || false;
                    s.node_count = r.node_count || 0;
                    s.error = r.error || '';
                }).catch(function () { }).finally(function () {
                    self.$set(self.testingStatus, key, false);
                    done++;
                    if (done === total) { self.batchTesting = false; self.fetchSources(); self.$message.success('批量测试完成，共 ' + total + ' 个'); }
                });
            });
        },
        batchDeleteFailed: function () {
            var self = this;
            var failed = this.selectedSources.filter(function (s) { return s.status === 404 || s.status === 0 || s.status >= 500 || (s.error && s.error.length > 0); });
            if (!failed.length) return this.$message.info('选中的源中没有失败的');
            this.$confirm('将删除 ' + failed.length + ' 个失败源，确认？', '提示', { type: 'warning' }).then(function () {
                var done = 0;
                failed.forEach(function (s) {
                    axios.delete('/api/sources', { data: sourcePayload(s) }).finally(function () {
                        done++;
                        if (done === failed.length) { self.$message.success('已删除 ' + done + ' 个失败源'); self.selectedSources = []; self.fetchSources(); }
                    });
                });
            }).catch(function () { });
        },
        batchDeleteSelected: function () {
            if (!this.selectedSources.length) return;
            var self = this;
            this.$confirm('将删除 ' + this.selectedSources.length + ' 个源，确认？', '提示', { type: 'warning' }).then(function () {
                var done = 0, total = self.selectedSources.length;
                self.selectedSources.forEach(function (s) {
                    axios.delete('/api/sources', { data: sourcePayload(s) }).finally(function () {
                        done++;
                        if (done === total) { self.$message.success('已删除 ' + done + ' 个源'); self.selectedSources = []; self.fetchSources(); }
                    });
                });
            }).catch(function () { });
        },

        // --- View data ---
        viewSourceData: function (row) {
            var self = this;
            this.sourceData = { url: row.url, name: row.name, proxies: [], total: 0, status: 0, latency: 0, proxy_groups: 0, error: '' };
            this.sourceDataVisible = true;
            this.sourceDataLoading = true;
            axios.get('/api/sources/data', { params: sourcePayload(row) }).then(function (r) {
                self.sourceData = Object.assign(self.sourceData, r.data);
            }).catch(function (e) { self.sourceData.error = '加载失败: ' + e; })
                .finally(function () { self.sourceDataLoading = false; });
        },

        tableRowClassName: function (ref) { return ref.row.status >= 500 ? 'invalid-row' : ''; }
    },
    template: `
<div>
  <!-- 批量操作栏 -->
  <div v-if="selectedSources.length" style="margin-bottom:12px;display:flex;align-items:center;gap:10px;padding:10px 16px;background:#ecf5ff;border-radius:4px;">
    <span style="font-size:13px;">已选 <b>{{ selectedSources.length }}</b> 项</span>
    <el-button size="mini" type="primary" icon="el-icon-connection" @click="batchTest" :loading="batchTesting">批量测试</el-button>
    <el-button size="mini" type="warning" icon="el-icon-delete" @click="batchDeleteFailed">删除失败源</el-button>
    <el-button size="mini" type="danger" icon="el-icon-delete" @click="batchDeleteSelected">删除选中</el-button>
    <el-button size="mini" @click="selectedSources=[]">取消选择</el-button>
  </div>

  <!-- 表格 -->
  <el-table ref="sourceTable" :data="sources" style="width:100%" v-loading="loading" size="small"
    :default-sort="{prop:'last_update',order:'descending'}" :row-class-name="tableRowClassName" @selection-change="onSourceSelect">
    <el-table-column type="selection" width="40"></el-table-column>
    <el-table-column prop="name" label="名称" width="130" sortable show-overflow-tooltip>
      <template slot-scope="scope">
        <span v-if="scope.row.name && scope.row.name !== scope.row.url">{{ scope.row.name }}</span>
        <span v-else class="hint">-</span>
      </template>
    </el-table-column>
    <el-table-column label="类型" width="92" align="center">
      <template slot-scope="scope">
        <el-tag size="mini" :type="scope.row.type === 'remote_url' ? 'primary' : 'success'">{{ sourceTypeText(scope.row.type) }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column prop="url" label="位置" sortable min-width="260" show-overflow-tooltip>
      <template slot-scope="scope"><span class="mono" style="font-size:12px;">{{ sourceLocationText(scope.row) }}</span></template>
    </el-table-column>
    <el-table-column label="启用" width="70" align="center">
      <template slot-scope="scope"><el-switch v-model="scope.row.enabled" size="mini" @change="toggleSourceEnabled(scope.row)"></el-switch></template>
    </el-table-column>
    <el-table-column label="客户端" width="80" align="center">
      <template slot-scope="scope"><span style="font-size:12px;">{{ scope.row.ua || 'Clash' }}</span></template>
    </el-table-column>
    <el-table-column label="状态" width="70" align="center" sortable>
      <template slot-scope="scope"><el-tag :type="getStatusTagType(scope.row.status)" size="mini" effect="dark">{{ getStatusText(scope.row.status) }}</el-tag></template>
    </el-table-column>
    <el-table-column prop="node_count" label="节点" width="72" align="center" sortable>
      <template slot-scope="scope"><span :class="scope.row.node_count > 0 ? '' : 'hint'">{{ scope.row.node_count || 0 }}</span></template>
    </el-table-column>
    <el-table-column label="延迟" width="80" align="center" sortable>
      <template slot-scope="scope"><span style="font-size:12px;">{{ scope.row.is_cached ? '⚡' : '' }}{{ scope.row.latency > 0 ? scope.row.latency + 'ms' : '-' }}</span></template>
    </el-table-column>
    <el-table-column label="操作" width="160" align="center" class-name="op-cell">
      <template slot-scope="scope">
        <el-tooltip content="查看节点"><el-button size="mini" icon="el-icon-view" circle @click="viewSourceData(scope.row)"></el-button></el-tooltip>
        <el-tooltip content="编辑"><el-button size="mini" icon="el-icon-edit" circle @click="openSourceDialog(scope.row)"></el-button></el-tooltip>
        <el-tooltip content="测试"><el-button size="mini" icon="el-icon-connection" circle @click="testSource(scope.row)" :loading="testingStatus[sourceKey(scope.row)]"></el-button></el-tooltip>
        <el-tooltip content="删除"><el-button size="mini" type="danger" icon="el-icon-delete" circle @click="deleteSource(scope.row)"></el-button></el-tooltip>
      </template>
    </el-table-column>
  </el-table>

  <!-- 添加订阅源对话框 -->
  <el-dialog title="添加订阅源" :visible.sync="addSourceVisible" width="560px">
    <el-form label-width="90px" size="small">
      <el-form-item label="名称" required><el-input v-model="addSourceForm.name" placeholder="如：69云、我的VPS"></el-input></el-form-item>
      <el-form-item label="类型">
        <el-radio-group v-model="addSourceForm.type" size="small">
          <el-radio-button label="remote_url">远程URL</el-radio-button>
          <el-radio-button label="local_uri">本地URI</el-radio-button>
          <el-radio-button label="local_yaml">本地YAML</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="URL" v-if="addSourceForm.type === 'remote_url'"><el-input v-model="addSourceForm.url" placeholder="https://..."></el-input></el-form-item>
      <el-form-item label="本地内容" v-else><el-input type="textarea" :rows="4" v-model="addSourceForm.content" placeholder="粘贴 ss://、vmess://、trojan:// 等 URI 或 Clash YAML"></el-input></el-form-item>
      <el-form-item label="启用"><el-switch v-model="addSourceForm.enabled"></el-switch></el-form-item>
      <el-form-item label="客户端模式">
        <el-select v-model="addSourceForm.ua" style="width:100%">
          <el-option label="Clash（推荐）" value="clash"></el-option>
          <el-option label="Mihomo" value="mihomo"></el-option>
          <el-option label="Surge" value="surge"></el-option>
          <el-option label="Shadowrocket" value="shadowrocket"></el-option>
          <el-option label="Quantumult" value="quantumult"></el-option>
          <el-option label="Loon" value="loon"></el-option>
          <el-option label="浏览器" value="browser"></el-option>
        </el-select>
        <div class="hint">部分机场只对特定客户端返回数据，Clash 兼容性最好。</div>
      </el-form-item>
    </el-form>
    <span slot="footer"><el-button @click="addSourceVisible=false">取消</el-button><el-button type="primary" @click="doAddSource">添加</el-button></span>
  </el-dialog>

  <!-- 编辑订阅源对话框 -->
  <el-dialog title="编辑订阅源" :visible.sync="sourceDialogVisible" width="560px">
    <el-form label-width="90px" size="small">
      <el-form-item label="名称" required><el-input v-model="sourceDialogForm.name" placeholder="如：69云、我的VPS"></el-input></el-form-item>
      <el-form-item label="类型"><el-input :value="sourceTypeText(sourceDialogForm.type)" disabled></el-input></el-form-item>
      <el-form-item label="位置"><el-input :value="sourceLocationText(sourceDialogForm)" disabled></el-input></el-form-item>
      <el-form-item label="启用"><el-switch v-model="sourceDialogForm.enabled"></el-switch></el-form-item>
      <el-form-item label="客户端模式">
        <el-select v-model="sourceDialogForm.ua" style="width:100%">
          <el-option label="Clash（推荐）" value="clash"></el-option>
          <el-option label="Mihomo" value="mihomo"></el-option>
          <el-option label="Surge" value="surge"></el-option>
          <el-option label="Shadowrocket" value="shadowrocket"></el-option>
          <el-option label="Quantumult" value="quantumult"></el-option>
          <el-option label="Loon" value="loon"></el-option>
          <el-option label="浏览器" value="browser"></el-option>
        </el-select>
        <div class="hint">修改后需点「测试」才能生效。</div>
      </el-form-item>
    </el-form>
    <span slot="footer"><el-button @click="sourceDialogVisible=false">取消</el-button><el-button type="primary" @click="saveSourceDialog">保存</el-button></span>
  </el-dialog>

  <!-- 查看源数据对话框 -->
  <el-dialog :title="'节点数据 - ' + (sourceData.name || '')" :visible.sync="sourceDataVisible" width="960px" top="5vh">
    <div v-if="sourceDataLoading" style="text-align:center;padding:40px;"><i class="el-icon-loading"></i> 加载中...</div>
    <div v-else-if="sourceData.error"><el-alert type="error" :closable="false">{{ sourceData.error }}</el-alert></div>
    <div v-else>
      <el-descriptions :column="4" border size="small" style="margin-bottom:12px;">
        <el-descriptions-item label="状态">{{ sourceData.status }}</el-descriptions-item>
        <el-descriptions-item label="延迟">{{ sourceData.latency }}ms</el-descriptions-item>
        <el-descriptions-item label="节点数">{{ sourceData.total }}</el-descriptions-item>
        <el-descriptions-item label="策略组">{{ sourceData.proxy_groups }}</el-descriptions-item>
      </el-descriptions>
      <el-table :data="sourceData.proxies" style="width:100%" max-height="500" size="mini" border>
        <el-table-column type="index" width="45" label="#"></el-table-column>
        <el-table-column prop="name" label="节点名" min-width="200" show-overflow-tooltip></el-table-column>
        <el-table-column prop="type" label="类型" width="70"><template slot-scope="scope"><el-tag size="mini">{{ scope.row.type }}</el-tag></template></el-table-column>
        <el-table-column prop="server" label="服务器" width="160" class-name="mono" show-overflow-tooltip></el-table-column>
        <el-table-column prop="port" label="端口" width="70"></el-table-column>
        <el-table-column label="加密" width="120" show-overflow-tooltip><template slot-scope="scope">{{ scope.row.cipher || '-' }}</template></el-table-column>
        <el-table-column label="TLS" width="50" align="center"><template slot-scope="scope">{{ scope.row.tls ? '✅' : '-' }}</template></el-table-column>
        <el-table-column label="UDP" width="50" align="center"><template slot-scope="scope">{{ scope.row.udp ? '✅' : '-' }}</template></el-table-column>
      </el-table>
    </div>
  </el-dialog>
</div>
`
});
