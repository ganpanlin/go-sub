// profile-list.js — 聚合订阅管理 tab：表格、编辑对话框、模拟预览、健康检测

Vue.component('profile-list', {
    data: function () {
        return {
            // Profile dialog
            profileDialogVisible: false,
            profileForm: {},
            // Simulation
            simulating: false,
            simDialogVisible: false,
            simResult: null,
            simLoading: false,
            simExpanded: [],
            // Health check
            hcDialogVisible: false,
            hcLoading: false,
            hcResults: [],
            hcProfile: null,
            hcDuration: 0,
            // Access stats
            profileStats: []
        };
    },
    computed: {
        profiles: function () { return this.$root.profiles; },
        routingProfiles: function () { return this.$root.routingProfiles; },
        sources: function () { return this.$root.sources; },
        loading: function () { return this.$root.loading; },
        sourceTransferData: function () {
            var self = this;
            return this.sources.map(function (s) {
                var location = sourceLocationText(s);
                var label = (s.name && s.name !== s.url) ? s.name : (location.length > 40 ? location.substring(0, 40) + '...' : location);
                return { id: sourceKey(s), url: location, label: label, name: s.name || '' };
            });
        }
    },
    methods: {
        // --- Data ---
        fetchProfiles: function () {
            var self = this;
            axios.get('/api/profiles').then(function (r) { self.$root.profiles = r.data || []; });
        },
        fetchProfileStats: function () {
            var self = this;
            axios.get('/api/profiles/stats').then(function (r) { self.profileStats = r.data || []; }).catch(function () { });
        },
        getProfileStat: function (id) {
            var s = this.profileStats.find(function (p) { return p.id === id; });
            return s || {};
        },
        getRoutingName: function (id) {
            var rp = this.routingProfiles.find(function (r) { return r.id === id; });
            return rp ? rp.name : id;
        },

        // --- Dialog ---
        openProfileDialog: function (row) {
            var p = row ? JSON.parse(JSON.stringify(row)) : {
                name: '', enabled: true, sources: [], routing_id: '',
                include: '', exclude: '公告|套餐|到期', type_filter: '', server_filter: '',
                rename_pattern: '{code}_{tag}', sort_by: 'region', script: '', overrides: null, source_prefix: 'name'
            };
            p.sources = this.normalizeProfileSources(p.sources || []);
            p.overridesText = p.overrides ? JSON.stringify(p.overrides, null, 2) : '';
            this.profileForm = p;
            this.profileDialogVisible = true;
        },
        normalizeProfileSources: function (list) {
            return list.map(function (ref) {
                var source = this.sources.find(function (s) { return s.id === ref || s.url === ref; });
                return source ? sourceKey(source) : ref;
            }.bind(this));
        },
        saveProfile: function () {
            var p = JSON.parse(JSON.stringify(this.profileForm));
            var self = this;
            if (!p.name) return this.$message.warning('名称不能为空');
            if (p.overridesText) {
                try { p.overrides = JSON.parse(p.overridesText); } catch (e) { return this.$message.error('JSON 格式错误'); }
            } else { p.overrides = null; }
            delete p.overridesText;
            var promise;
            if (p.id) {
                promise = axios.put('/api/profiles?id=' + p.id, p).catch(function (err) {
                    if (err.response && err.response.status === 404) {
                        delete p.id;
                        return axios.post('/api/profiles', p).then(function (resp) { return axios.put('/api/profiles?id=' + resp.data.id, p); });
                    }
                    throw err;
                });
            } else {
                promise = axios.post('/api/profiles', { name: p.name }).then(function (resp) { return axios.put('/api/profiles?id=' + resp.data.id, p); });
            }
            promise.then(function () {
                self.$message.success('保存成功'); self.profileDialogVisible = false; self.fetchProfiles();
            }).catch(function (e) { self.$message.error('保存失败: ' + apiErrorMessage(e)); });
        },
        deleteProfile: function (id) {
            var self = this;
            this.$confirm('确认删除？', '提示', { type: 'warning' }).then(function () {
                axios.delete('/api/profiles?id=' + id).then(function () { self.$message.success('已删除'); self.fetchProfiles(); });
            }).catch(function () { });
        },

        // --- Subscription URL ---
        subUrl: function (token) { return location.origin + '/sub/' + token; },
        openSub: function (token) { window.open(this.subUrl(token), '_blank'); },
        copySub: function (token) {
            navigator.clipboard.writeText(this.subUrl(token));
            this.$message.success('已复制');
        },
        resetToken: function (row) {
            var self = this;
            this.$confirm('重置后旧链接立即失效', '重置 Token', { type: 'warning' }).then(function () {
                axios.put('/api/profiles?id=' + row.id, { reset_token: true }).then(function () {
                    self.$message.success('Token 已重置'); self.fetchProfiles();
                });
            }).catch(function () { });
        },

        // --- Simulation ---
        simulateProfile: function (row) {
            var id = row ? row.id : this.profileForm.id;
            if (!id) return this.$message.warning('请先保存');
            var self = this;
            this.simulating = true; this.simDialogVisible = true; this.simLoading = true; this.simResult = null;
            axios.post('/api/simulate', { id: id }).then(function (r) {
                self.simResult = r.data; self.simExpanded = [];
            }).catch(function (e) {
                self.$message.error('模拟失败: ' + apiErrorMessage(e)); self.simDialogVisible = false;
            }).finally(function () { self.simulating = false; self.simLoading = false; });
        },

        // --- Health Check ---
        openHealthCheck: function (row) {
            this.hcProfile = row;
            this.hcResults = [];
            this.hcDialogVisible = true;
            this.hcLoading = true;
            this.hcDuration = 0;
            var self = this;
            axios.post('/api/health-check', { profile_id: row.id }).then(function (r) {
                var d = r.data || {};
                self.hcResults = d.results || [];
                self.hcDuration = d.duration_ms || 0;
            }).catch(function (e) {
                self.$message.error('健康检测失败: ' + apiErrorMessage(e));
            }).finally(function () { self.hcLoading = false; });
        },
        hcTagType: function (r) {
            if (!r.alive) return 'danger';
            if (r.latency > 800) return 'warning';
            return 'success';
        },

        // --- Transfer filter ---
        transferFilter: function (query, item) {
            var q = query.toLowerCase();
            return item.label.toLowerCase().indexOf(q) > -1 || item.url.toLowerCase().indexOf(q) > -1 || (item.name && item.name.toLowerCase().indexOf(q) > -1);
        },

        formatTime: function (t) {
            if (!t) return '-';
            return t.replace('T', ' ').substring(0, 19);
        }
    },
    template: `
<div>
  <!-- 工具栏 -->
  <div style="margin-bottom:12px;display:flex;justify-content:space-between;align-items:center;">
    <el-button type="primary" size="small" @click="openProfileDialog()">新建聚合订阅</el-button>
    <el-button size="small" @click="fetchProfileStats" icon="el-icon-data-line">刷新统计</el-button>
  </div>

  <!-- 表格 -->
  <el-table :data="profiles" style="width:100%" v-loading="loading" size="small">
    <el-table-column prop="name" label="名称" width="130" show-overflow-tooltip></el-table-column>
    <el-table-column label="分流" width="90">
      <template slot-scope="scope"><span style="font-size:12px;">{{ scope.row.routing_id ? getRoutingName(scope.row.routing_id) : '默认' }}</span></template>
    </el-table-column>
    <el-table-column label="规则" min-width="180">
      <template slot-scope="scope">
        <div class="hint" style="line-height:1.4;">
          In: {{ scope.row.include || '-' }} / Ex: {{ scope.row.exclude || '-' }}
          <span v-if="scope.row.type_filter"> / T: {{ scope.row.type_filter }}</span>
          <span v-if="scope.row.source_prefix && scope.row.source_prefix !== 'off'"> / 前缀:{{ scope.row.source_prefix }}</span>
        </div>
      </template>
    </el-table-column>
    <el-table-column label="订阅链接" min-width="160">
      <template slot-scope="scope"><div class="sub-url">{{ subUrl(scope.row.token) }}</div></template>
    </el-table-column>
    <el-table-column label="访问" width="90" align="center">
      <template slot-scope="scope">
        <span v-if="getProfileStat(scope.row.id).access_count" style="font-size:12px;">
          {{ getProfileStat(scope.row.id).access_count }}次
        </span>
        <span v-else class="hint">-</span>
      </template>
    </el-table-column>
    <el-table-column label="操作" width="240" align="center" class-name="op-cell">
      <template slot-scope="scope">
        <el-tooltip content="健康检测"><el-button size="mini" icon="el-icon-cpu" circle @click="openHealthCheck(scope.row)"></el-button></el-tooltip>
        <el-tooltip content="模拟预览"><el-button size="mini" icon="el-icon-monitor" circle @click="simulateProfile(scope.row)"></el-button></el-tooltip>
        <el-tooltip content="编辑"><el-button size="mini" type="primary" icon="el-icon-edit" circle @click="openProfileDialog(scope.row)"></el-button></el-tooltip>
        <el-tooltip content="打开"><el-button size="mini" icon="el-icon-link" circle @click="openSub(scope.row.token)"></el-button></el-tooltip>
        <el-tooltip content="复制链接"><el-button size="mini" icon="el-icon-document-copy" circle @click="copySub(scope.row.token)"></el-button></el-tooltip>
        <el-tooltip content="重置Token"><el-button size="mini" icon="el-icon-refresh-right" circle @click="resetToken(scope.row)"></el-button></el-tooltip>
        <el-tooltip content="删除"><el-button size="mini" type="danger" icon="el-icon-delete" circle @click="deleteProfile(scope.row.id)"></el-button></el-tooltip>
      </template>
    </el-table-column>
  </el-table>

  <!-- 聚合订阅编辑对话框 -->
  <el-dialog :title="profileForm.id ? '编辑聚合订阅' : '新建聚合订阅'" :visible.sync="profileDialogVisible" width="920px" top="5vh">
    <el-form label-width="110px" size="small">
      <el-form-item label="启用"><el-switch v-model="profileForm.enabled"></el-switch></el-form-item>
      <el-form-item label="名称" required><el-input v-model="profileForm.name" placeholder="例如：HK + JP 优选"></el-input></el-form-item>
      <el-form-item label="分流规则">
        <el-select v-model="profileForm.routing_id" clearable style="width:100%" placeholder="不选则使用默认分流规则">
          <el-option v-for="rp in routingProfiles" :key="rp.id" :label="rp.name" :value="rp.id"></el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="订阅源">
        <el-transfer v-model="profileForm.sources" :data="sourceTransferData" :titles="['可选源','已选源']"
          :props="{key:'id',label:'label'}" filterable :filter-method="transferFilter" filter-placeholder="搜索源名称或URL" style="width:100%">
          <template slot-scope="{option}"><span :title="option.url">{{ option.label }}</span></template>
        </el-transfer>
        <div class="hint">留空 = 聚合所有已配置订阅源。</div>
      </el-form-item>
      <el-row :gutter="12">
        <el-col :span="12"><el-form-item label="包含正则"><el-input v-model="profileForm.include" placeholder="如：(?i)香港|HK|🇭🇰"></el-input></el-form-item></el-col>
        <el-col :span="12"><el-form-item label="去除正则"><el-input v-model="profileForm.exclude" placeholder="如：公告|套餐|到期"></el-input></el-form-item></el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="8"><el-form-item label="类型过滤"><el-input v-model="profileForm.type_filter" placeholder="vless|vmess|ss"></el-input></el-form-item></el-col>
        <el-col :span="8">
          <el-form-item label="Server">
            <el-select v-model="profileForm.server_filter" clearable style="width:100%">
              <el-option label="不限" value=""></el-option><el-option label="只要 IP" value="ip"></el-option><el-option label="只要域名" value="domain"></el-option>
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="排序">
            <el-select v-model="profileForm.sort_by" clearable style="width:100%">
              <el-option label="不排序" value=""></el-option><el-option label="按区域" value="region"></el-option><el-option label="按名称" value="name"></el-option><el-option label="按类型" value="type"></el-option>
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="12">
          <el-form-item label="节点名前缀">
            <el-select v-model="profileForm.source_prefix" style="width:100%">
              <el-option label="不加前缀" value="off"></el-option><el-option label="用订阅源名称" value="name"></el-option><el-option label="用域名" value="domain"></el-option>
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12"><el-form-item label="重命名模板"><el-input v-model="profileForm.rename_pattern" placeholder="{code}_{tag}"></el-input></el-form-item></el-col>
      </el-row>
      <el-form-item label="字段覆写"><el-input type="textarea" :rows="2" v-model="profileForm.overridesText" placeholder='{"tls":true,"sni":"example.com"}'></el-input></el-form-item>
      <el-form-item label="JS脚本">
        <el-input class="script-box" type="textarea" :rows="8" v-model="profileForm.script" placeholder='function transform(node) { return node; }'></el-input>
        <div class="hint">transform(node)：返回 false 排除；返回 true 保留；返回对象覆写字段。</div>
      </el-form-item>
    </el-form>
    <span slot="footer">
      <el-button @click="profileDialogVisible=false">取消</el-button>
      <el-button type="warning" @click="simulateProfile" :loading="simulating" v-if="profileForm.id">模拟</el-button>
      <el-button type="primary" @click="saveProfile">保存</el-button>
    </span>
  </el-dialog>

  <!-- 模拟预览对话框 -->
  <el-dialog title="模拟预览" :visible.sync="simDialogVisible" width="960px" top="3vh">
    <div v-if="simLoading" style="text-align:center;padding:40px;"><i class="el-icon-loading"></i> 正在生成配置...</div>
    <div v-else-if="simResult">
      <el-alert v-if="simResult.errors && simResult.errors.length" type="error" :closable="false" style="margin-bottom:12px;"><div v-for="e in simResult.errors" :key="e">❌ {{ e }}</div></el-alert>
      <el-alert v-if="simResult.warnings && simResult.warnings.length" type="warning" :closable="false" style="margin-bottom:12px;"><div v-for="w in simResult.warnings" :key="w">⚠️ {{ w }}</div></el-alert>
      <el-alert v-if="simResult.ok" type="success" :closable="false" style="margin-bottom:12px;">✅ 配置验证通过</el-alert>
      <el-descriptions :column="3" border size="small" style="margin-bottom:12px;">
        <el-descriptions-item label="节点">{{ simResult.summary.total_nodes }} → {{ simResult.summary.filtered_nodes }}</el-descriptions-item>
        <el-descriptions-item label="策略组">{{ simResult.summary.proxy_groups }}</el-descriptions-item>
        <el-descriptions-item label="规则">{{ simResult.summary.rules }} ({{ simResult.summary.rule_providers }} 远程)</el-descriptions-item>
      </el-descriptions>
      <div v-if="simResult.sources && simResult.sources.length" style="margin-bottom:8px;">
        <strong>源：</strong>
        <el-tag v-for="s in simResult.sources" :key="s.url" size="mini" :type="s.error ? 'danger' : 'success'" style="margin:2px;">{{ s.name || s.url }} ({{ s.node_count }}){{ s.error ? ' ✗' : '' }}</el-tag>
      </div>
      <el-collapse v-model="simExpanded">
        <el-collapse-item title="YAML 预览" name="yaml">
          <pre class="mono" style="max-height:500px;overflow:auto;background:#fafafa;padding:12px;border-radius:4px;font-size:12px;line-height:1.4;">{{ simResult.yaml_preview }}</pre>
        </el-collapse-item>
      </el-collapse>
    </div>
  </el-dialog>

  <!-- 健康检测对话框 -->
  <el-dialog :title="'健康检测 - ' + (hcProfile ? hcProfile.name : '')" :visible.sync="hcDialogVisible" width="900px" top="5vh">
    <div v-if="hcLoading" style="text-align:center;padding:40px;"><i class="el-icon-loading"></i> 正在检测节点连通性...</div>
    <div v-else>
      <el-descriptions :column="3" border size="small" style="margin-bottom:12px;">
        <el-descriptions-item label="总数">{{ hcResults.length }}</el-descriptions-item>
        <el-descriptions-item label="存活">{{ hcResults.filter(function(r){return r.alive}).length }}</el-descriptions-item>
        <el-descriptions-item label="耗时">{{ hcDuration }}ms</el-descriptions-item>
      </el-descriptions>
      <el-table :data="hcResults" style="width:100%" max-height="500" size="mini" border>
        <el-table-column type="index" width="45" label="#"></el-table-column>
        <el-table-column prop="name" label="节点" min-width="200" show-overflow-tooltip></el-table-column>
        <el-table-column prop="server" label="服务器" width="160" show-overflow-tooltip></el-table-column>
        <el-table-column prop="port" label="端口" width="70"></el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template slot-scope="scope">
            <el-tag :type="hcTagType(scope.row)" size="mini">{{ scope.row.alive ? '✓' : '✗' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="延迟" width="90" align="center" sortable>
          <template slot-scope="scope">
            <span :style="{color: scope.row.latency > 800 ? '#E6A23C' : scope.row.alive ? '#67C23A' : '#F56C6C'}">
              {{ scope.row.latency >= 0 ? scope.row.latency + 'ms' : '超时' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="error" label="错误" min-width="120" show-overflow-tooltip>
          <template slot-scope="scope"><span class="hint">{{ scope.row.error || '-' }}</span></template>
        </el-table-column>
      </el-table>
    </div>
  </el-dialog>
</div>
`
});
