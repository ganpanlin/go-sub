// config-io.js — 配置导入导出对话框

Vue.component('config-io', {
    data: function () {
        return {
            importVisible: false,
            importLoading: false,
            importMode: 'overwrite',
            importFile: null,
            importJson: ''
        };
    },
    methods: {
        exportConfig: function () {
            var self = this;
            axios.get('/api/config/export').then(function (r) {
                var data = JSON.stringify(r.data || {}, null, 2);
                var blob = new Blob([data], { type: 'application/json' });
                var url = URL.createObjectURL(blob);
                var a = document.createElement('a');
                a.href = url;
                a.download = 'proxy-filter-config-' + new Date().toISOString().slice(0, 10) + '.json';
                a.click();
                URL.revokeObjectURL(url);
                self.$message.success('配置已导出');
            }).catch(function (e) {
                self.$message.error('导出失败: ' + apiErrorMessage(e));
            });
        },
        openImportDialog: function () {
            this.importFile = null;
            this.importJson = '';
            this.importMode = 'overwrite';
            this.importVisible = true;
        },
        handleImportFile: function (e) {
            var self = this;
            var file = e.target.files[0];
            if (!file) return;
            var reader = new FileReader();
            reader.onload = function (ev) {
                self.importJson = ev.target.result;
            };
            reader.readAsText(file);
        },
        doImport: function () {
            if (!this.importJson) return this.$message.warning('请选择文件或粘贴 JSON');
            var self = this;
            this.importLoading = true;
            axios.post('/api/config/import', {
                data: this.importJson,
                mode: this.importMode
            }).then(function (r) {
                self.$message.success((r.data || {}).message || '导入成功');
                self.importVisible = false;
                // Refresh all data
                self.$root.fetchAll();
            }).catch(function (e) {
                self.$message.error('导入失败: ' + apiErrorMessage(e));
            }).finally(function () {
                self.importLoading = false;
            });
        }
    },
    template: `
<span>
  <el-dialog title="导入配置" :visible.sync="importVisible" width="560px">
    <el-form label-width="90px" size="small">
      <el-form-item label="选择文件">
        <input type="file" accept=".json" @change="handleImportFile" style="width:100%;" />
      </el-form-item>
      <el-form-item label="或粘贴JSON">
        <el-input type="textarea" :rows="8" v-model="importJson" placeholder="粘贴之前导出的 JSON 数据"></el-input>
      </el-form-item>
      <el-form-item label="导入模式">
        <el-radio-group v-model="importMode">
          <el-radio label="overwrite">覆盖（替换所有数据）</el-radio>
        </el-radio-group>
        <div class="hint">导入后需重启服务才能完全生效。</div>
      </el-form-item>
    </el-form>
    <span slot="footer">
      <el-button @click="importVisible=false">取消</el-button>
      <el-button type="primary" @click="doImport" :loading="importLoading">导入</el-button>
    </span>
  </el-dialog>
</span>
`
});
