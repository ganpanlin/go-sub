// routing.js — 分流规则管理 tab：折叠面板、规则编辑、规则库

Vue.component('routing-manager', {
    data: function () {
        return {
            expandedRouting: [],
            // Routing dialog
            routingDialogVisible: false,
            routingForm: { name: '', rules: [] },
            // Rule catalog
            catalogChecked: {},
            catalogFilter: '',
            // Rule detail
            ruleDetailVisible: false,
            ruleDetailData: null
        };
    },
    computed: {
        routingProfiles: function () { return this.$root.routingProfiles; },
        ruleCatalog: function () { return this.$root.ruleCatalog; },
        hasCheckedCatalog: function () { return this.checkedCatalogCount > 0; },
        checkedCatalogCount: function () {
            var count = 0;
            Object.values(this.catalogChecked).forEach(function (cat) {
                Object.values(cat).forEach(function (v) { if (v) count++; });
            });
            return count;
        },
        filteredCatalog: function () {
            if (!this.catalogFilter) return this.ruleCatalog;
            var q = this.catalogFilter.toLowerCase();
            return this.ruleCatalog.map(function (cat) {
                var rules = cat.rules.filter(function (r) { return r.name.toLowerCase().indexOf(q) > -1 || r.id.toLowerCase().indexOf(q) > -1; });
                if (!rules.length) return null;
                return { id: cat.id, name: cat.name, gfw: cat.gfw, rules: rules };
            }).filter(Boolean);
        }
    },
    methods: {
        // --- Data ---
        fetchRouting: function () {
            var self = this;
            axios.get('/api/routing').then(function (r) {
                self.$root.routingProfiles = r.data || [];
                if (self.$root.routingProfiles.length && !self.expandedRouting.length) {
                    self.expandedRouting = [self.$root.routingProfiles[0].id];
                }
            });
        },

        // --- Dialog ---
        openRoutingDialog: function (rp) {
            if (rp) {
                this.routingForm = JSON.parse(JSON.stringify(rp));
                (this.routingForm.rules || []).forEach(function (r) {
                    r.urls = r.urls || []; r.payload = r.payload || []; r.extraProxies = r.extraProxies || [];
                });
            } else {
                this.routingForm = { name: '', rules: [] };
            }
            this.catalogChecked = {};
            this.catalogFilter = '';
            this.routingDialogVisible = true;
        },
        addRuleItem: function () {
            this.routingForm.rules = this.routingForm.rules || [];
            this.routingForm.rules.push({ name: '', gfw: false, urls: [], payload: [], extraProxies: [] });
        },
        moveRule: function (index, dir) {
            var rules = this.routingForm.rules;
            var target = index + dir;
            if (target < 0 || target >= rules.length) return;
            var temp = rules[index];
            this.$set(rules, index, rules[target]);
            this.$set(rules, target, temp);
        },
        openRuleDetail: function (index, row) {
            if (!row.urls) this.$set(row, 'urls', []);
            if (!row.payload) this.$set(row, 'payload', []);
            if (!row.extraProxies) this.$set(row, 'extraProxies', []);
            this.ruleDetailData = row;
            this.ruleDetailVisible = true;
        },
        urlDisplayName: function (url) {
            if (!url) return '';
            var parts = url.split('/');
            var yaml = parts[parts.length - 1];
            return yaml.replace('.yaml', '').replace('_Classical', '');
        },
        saveRouting: function () {
            var rp = JSON.parse(JSON.stringify(this.routingForm));
            var self = this;
            if (!rp.name) return this.$message.warning('名称不能为空');
            rp.rules.forEach(function (r) {
                r.urls = (r.urls || []).filter(function (u) { return u.trim(); });
                r.payload = (r.payload || []).filter(function (p) { return p.trim(); });
                r.extraProxies = (r.extraProxies || []).filter(function (e) { return e.trim(); });
            });
            rp.rules = rp.rules.filter(function (r) { return r.name.trim(); });
            var req = rp.id ? axios.put('/api/routing?id=' + rp.id, rp) : axios.post('/api/routing', rp);
            req.then(function () {
                self.$message.success('保存成功'); self.routingDialogVisible = false; self.fetchRouting();
            }).catch(function (e) { self.$message.error('保存失败: ' + apiErrorMessage(e)); });
        },
        deleteRouting: function (id) {
            var self = this;
            this.$confirm('确认删除？关联的聚合订阅将回退为默认规则。', '提示', { type: 'warning' }).then(function () {
                axios.delete('/api/routing?id=' + id).then(function () { self.$message.success('已删除'); self.fetchRouting(); });
            }).catch(function () { });
        },

        // --- Catalog ---
        isCatalogChecked: function (catId, ruleId) {
            return !!(this.catalogChecked[catId] && this.catalogChecked[catId][ruleId]);
        },
        toggleCatalog: function (catId, ruleId, val) {
            if (!this.catalogChecked[catId]) this.$set(this.catalogChecked, catId, {});
            this.$set(this.catalogChecked[catId], ruleId, val);
        },
        isCatAllChecked: function (cat) {
            return cat.rules.length > 0 && cat.rules.every(function (r) { return this.isCatalogChecked(cat.id, r.id) || this.isRuleAlreadyAdded(r); }.bind(this));
        },
        isCatPartial: function (cat) {
            var checked = cat.rules.filter(function (r) { return this.isCatalogChecked(cat.id, r.id); }.bind(this)).length;
            return checked > 0 && !this.isCatAllChecked(cat);
        },
        catCheckedCount: function (cat) {
            return cat.rules.filter(function (r) { return this.isCatalogChecked(cat.id, r.id); }.bind(this)).length;
        },
        toggleCatAll: function (cat, val) {
            var self = this;
            if (!this.catalogChecked[cat.id]) this.$set(this.catalogChecked, cat.id, {});
            cat.rules.forEach(function (r) {
                if (!self.isRuleAlreadyAdded(r)) { self.$set(self.catalogChecked[cat.id], r.id, val); }
            });
        },
        isRuleAlreadyAdded: function (rule) {
            var rules = this.routingForm.rules || [];
            var ruleUrls = rule.urls || (rule.url ? [rule.url] : []);
            if (ruleUrls.length) return rules.some(function (r) { return (r.urls || []).some(function (u) { return ruleUrls.indexOf(u) >= 0; }); });
            if (rule.payload && rule.payload.length) return rules.some(function (r) {
                var p = r.payload || [];
                return rule.payload.every(function (rp) { return p.indexOf(rp) >= 0; });
            });
            return false;
        },
        addCatalogRules: function () {
            var self = this;
            var added = [];
            this.ruleCatalog.forEach(function (cat) {
                var checked = self.catalogChecked[cat.id];
                if (!checked) return;
                cat.rules.forEach(function (rule) {
                    if (!checked[rule.id]) return;
                    var item = { name: rule.name, gfw: cat.gfw, urls: [], payload: [], extraProxies: [] };
                    if (rule.urls && rule.urls.length) item.urls = rule.urls.slice();
                    else if (rule.url) item.urls = [rule.url];
                    if (rule.payload) item.payload = rule.payload.slice();
                    self.routingForm.rules.push(item);
                    added.push(rule.name);
                });
            });
            this.catalogChecked = {};
            if (added.length) this.$message.success('已添加: ' + added.join(', '));
        }
    },
    template: `
<div>
  <div style="margin-bottom:12px;display:flex;justify-content:space-between;align-items:center;">
    <span class="hint">定义 proxy-groups / rule-providers / rules 的生成方式。</span>
    <el-button type="primary" size="small" @click="openRoutingDialog()">新建分流规则</el-button>
  </div>

  <el-collapse v-model="expandedRouting">
    <el-collapse-item v-for="rp in routingProfiles" :key="rp.id" :name="rp.id">
      <template slot="title">
        <span style="font-weight:500;">{{ rp.name }}</span>
        <el-tag v-if="rp.id === 'default'" size="mini" type="info" style="margin-left:8px;">系统默认</el-tag>
        <el-tag size="mini" style="margin-left:8px;">{{ rp.rules.length }} 条</el-tag>
        <span style="margin-left:auto;margin-right:10px;">
          <el-button size="mini" type="primary" @click.stop="openRoutingDialog(rp)">编辑</el-button>
          <el-button v-if="rp.id !== 'default'" size="mini" type="danger" @click.stop="deleteRouting(rp.id)">删除</el-button>
        </span>
      </template>
      <el-table :data="rp.rules" size="mini" border>
        <el-table-column prop="name" label="策略组" width="150"></el-table-column>
        <el-table-column label="GFW" width="60" align="center">
          <template slot-scope="scope"><el-tag :type="scope.row.gfw ? 'danger' : 'success'" size="mini">{{ scope.row.gfw ? '墙' : '直' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="远程规则集" min-width="280">
          <template slot-scope="scope">
            <div v-for="u in (scope.row.urls || [])" :key="u" class="mono" style="font-size:11px;word-break:break-all;">{{ u }}</div>
            <span v-if="!scope.row.urls || !scope.row.urls.length" class="hint">-</span>
          </template>
        </el-table-column>
        <el-table-column label="内联规则" min-width="180">
          <template slot-scope="scope">
            <el-tag v-for="p in (scope.row.payload || []).slice(0,3)" :key="p" size="mini" class="rule-tag">{{ p }}</el-tag>
            <el-tag v-if="(scope.row.payload || []).length > 3" size="mini" type="info">+{{ scope.row.payload.length - 3 }}</el-tag>
            <span v-if="!scope.row.payload || !scope.row.payload.length" class="hint">-</span>
          </template>
        </el-table-column>
        <el-table-column label="额外" width="80">
          <template slot-scope="scope">
            <el-tag v-for="e in (scope.row.extraProxies || [])" :key="e" size="mini" type="warning">{{ e }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-collapse-item>
  </el-collapse>

  <!-- ====== 分流规则编辑对话框 ====== -->
  <el-dialog :title="routingForm.id ? '编辑分流规则' : '新建分流规则'" :visible.sync="routingDialogVisible" width="1100px" top="3vh">
    <el-form label-width="70px" size="small" class="rule-edit-form">
      <el-form-item label="名称" required><el-input v-model="routingForm.name" placeholder="例如：默认分流规则"></el-input></el-form-item>
    </el-form>

    <!-- 已选规则 -->
    <div style="margin-bottom:8px;">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:6px;">
        <span style="font-weight:600;font-size:14px;">📋 已选规则 <el-tag size="mini" style="margin-left:4px;">{{ (routingForm.rules||[]).length }}</el-tag></span>
        <div style="display:flex;gap:6px;">
          <el-button size="mini" @click="addRuleItem">+ 自定义规则</el-button>
          <el-button size="mini" type="danger" @click="routingForm.rules=[]" v-if="routingForm.rules.length">清空全部</el-button>
        </div>
      </div>
      <el-table :data="routingForm.rules" size="mini" border max-height="280" style="width:100%">
        <el-table-column type="index" width="40" label="#"></el-table-column>
        <el-table-column label="策略组名" width="160"><template slot-scope="scope"><el-input v-model="scope.row.name" size="mini" placeholder="策略组名"></el-input></template></el-table-column>
        <el-table-column label="代理" width="60" align="center"><template slot-scope="scope"><el-switch v-model="scope.row.gfw" size="mini" active-color="#409eff" inactive-color="#67c23a"></el-switch></template></el-table-column>
        <el-table-column label="来源" min-width="300">
          <template slot-scope="scope">
            <div v-if="scope.row.urls && scope.row.urls.length"><el-tag v-for="(u,i) in scope.row.urls" :key="i" size="mini" type="info" style="margin:1px;">{{ urlDisplayName(u) }}</el-tag></div>
            <div v-if="scope.row.payload && scope.row.payload.length"><el-tag v-for="(p,i) in scope.row.payload" :key="'p'+i" size="mini" type="warning" style="margin:1px;">{{ p }}</el-tag></div>
            <span v-if="scope.row.extraProxies && scope.row.extraProxies.length" style="margin-left:4px;"><el-tag v-for="e in scope.row.extraProxies" :key="e" size="mini" type="danger">{{ e }}</el-tag></span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="110" align="center">
          <template slot-scope="scope">
            <el-tooltip content="编辑"><el-button size="mini" icon="el-icon-edit" circle @click="openRuleDetail(scope.$index, scope.row)"></el-button></el-tooltip>
            <el-tooltip content="上移" v-if="scope.$index > 0"><el-button size="mini" icon="el-icon-top" circle @click="moveRule(scope.$index, -1)"></el-button></el-tooltip>
            <el-tooltip content="删除"><el-button size="mini" type="danger" icon="el-icon-delete" circle @click="routingForm.rules.splice(scope.$index,1)"></el-button></el-tooltip>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 规则库 -->
    <div style="margin-bottom:12px;">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px;">
        <span style="font-weight:600;font-size:14px;">📦 选择规则</span>
        <div style="display:flex;gap:6px;align-items:center;">
          <el-input size="mini" style="width:160px;" v-model="catalogFilter" placeholder="搜索规则..." clearable prefix-icon="el-icon-search"></el-input>
          <el-button size="mini" type="success" @click="addCatalogRules" :disabled="!hasCheckedCatalog">加入已选 ({{ checkedCatalogCount }})</el-button>
        </div>
      </div>
      <div style="max-height:320px;overflow-y:auto;border:1px solid #ebeef5;border-radius:4px;padding:8px;background:#fafafa;">
        <div style="display:flex;flex-wrap:wrap;gap:6px;">
          <div v-for="cat in filteredCatalog" :key="cat.id" style="width:calc(50% - 3px);margin-bottom:4px;border:1px solid #e4e7ed;border-radius:4px;background:#fff;">
            <div style="display:flex;align-items:center;justify-content:space-between;padding:5px 10px;border-bottom:1px solid #f2f2f2;">
              <label style="font-weight:500;font-size:13px;cursor:pointer;display:flex;align-items:center;gap:5px;">
                <el-checkbox :value="isCatAllChecked(cat)" :indeterminate="isCatPartial(cat)" @change="function(v){toggleCatAll(cat,v)}"></el-checkbox>
                {{ cat.name }}
              </label>
              <el-tag size="mini" :type="catCheckedCount(cat) > 0 ? 'success' : 'info'">{{ catCheckedCount(cat) }}/{{ cat.rules.length }}</el-tag>
            </div>
            <div style="padding:6px 10px;display:flex;flex-wrap:wrap;gap:4px 10px;">
              <el-checkbox v-for="rule in cat.rules" :key="rule.id"
                :value="isCatalogChecked(cat.id, rule.id)"
                :disabled="isRuleAlreadyAdded(rule)"
                @change="function(v){toggleCatalog(cat.id, rule.id, v, rule)}">
                {{ rule.name }}
                <span v-if="isRuleAlreadyAdded(rule)" style="color:#c0c4cc;font-size:11px;">✓</span>
              </el-checkbox>
            </div>
          </div>
        </div>
        <div v-if="!filteredCatalog.length" style="text-align:center;padding:20px;color:#909399;">暂无匹配规则</div>
      </div>
    </div>

    <!-- 单条规则编辑 -->
    <el-dialog title="编辑规则详情" :visible.sync="ruleDetailVisible" width="700px" append-to-body>
      <div v-if="ruleDetailData">
        <el-form label-width="100px" size="small">
          <el-form-item label="策略组名"><el-input v-model="ruleDetailData.name"></el-input></el-form-item>
          <el-form-item label="走代理"><el-switch v-model="ruleDetailData.gfw"></el-switch></el-form-item>
          <el-form-item label="远程规则URL">
            <div v-for="(u,i) in ruleDetailData.urls" :key="'u'+i" style="display:flex;gap:4px;margin-bottom:4px;">
              <el-input v-model="ruleDetailData.urls[i]" class="mono" placeholder="https://..."></el-input>
              <el-button size="mini" type="danger" icon="el-icon-delete" circle @click="ruleDetailData.urls.splice(i,1)"></el-button>
            </div>
            <el-button size="mini" @click="$set(ruleDetailData,'urls',ruleDetailData.urls.concat(['']))">+ URL</el-button>
          </el-form-item>
          <el-form-item label="内联规则">
            <div v-for="(p,i) in ruleDetailData.payload" :key="'p'+i" style="display:flex;gap:4px;margin-bottom:4px;">
              <el-input v-model="ruleDetailData.payload[i]" class="mono" placeholder="DOMAIN-SUFFIX,example.com"></el-input>
              <el-button size="mini" type="danger" icon="el-icon-delete" circle @click="ruleDetailData.payload.splice(i,1)"></el-button>
            </div>
            <el-button size="mini" @click="$set(ruleDetailData,'payload',ruleDetailData.payload.concat(['']))">+ 规则</el-button>
            <div class="hint">只写匹配条件，如 DOMAIN-SUFFIX,google.com 或 IP-CIDR,10.0.0.0/8</div>
          </el-form-item>
          <el-form-item label="额外代理名">
            <div v-for="(e,i) in ruleDetailData.extraProxies" :key="'e'+i" style="display:flex;gap:4px;margin-bottom:4px;">
              <el-input v-model="ruleDetailData.extraProxies[i]" placeholder="REJECT"></el-input>
              <el-button size="mini" type="danger" icon="el-icon-delete" circle @click="ruleDetailData.extraProxies.splice(i,1)"></el-button>
            </div>
            <el-button size="mini" @click="$set(ruleDetailData,'extraProxies',ruleDetailData.extraProxies.concat(['']))">+ 代理</el-button>
          </el-form-item>
        </el-form>
      </div>
      <span slot="footer"><el-button @click="ruleDetailVisible=false">关闭</el-button></span>
    </el-dialog>

    <span slot="footer">
      <el-button @click="routingDialogVisible=false">取消</el-button>
      <el-button type="primary" @click="saveRouting">保存</el-button>
    </span>
  </el-dialog>
</div>
`
});
