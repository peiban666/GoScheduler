<template>
  <el-container>
    <system-sidebar></system-sidebar>
    <el-main>
      <notification-tab></notification-tab>
      <div class="webhook-toolbar">
        <el-button type="primary" icon="el-icon-plus" @click="createWebhook">新建 Webhook</el-button>
        <span class="webhook-hint">不同任务可以选择不同的 Webhook，也可以同时通知多个机器人。</span>
      </div>
      <el-table :data="endpoints" border empty-text="还没有 Webhook，点击上方按钮新建">
        <el-table-column prop="name" label="名称" min-width="160"></el-table-column>
        <el-table-column label="平台" min-width="130">
          <template slot-scope="scope">{{ providerLabel(scope.row.provider) }}</template>
        </el-table-column>
        <el-table-column label="加签" width="90">
          <template slot-scope="scope">{{ scope.row.sign_enabled ? '已开启' : '未开启' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="240">
          <template slot-scope="scope">
            <el-button type="text" @click="editWebhook(scope.row)">编辑</el-button>
            <el-button type="text" :loading="testingId === scope.row.id" :disabled="testing" @click="testSaved(scope.row)">测试发送</el-button>
            <el-button type="text" class="webhook-delete" @click="removeWebhook(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-form v-if="editing" ref="form" :model="form" :rules="formRules" label-width="100px" class="webhook-form">
        <h3>{{ form.id === -1 ? '新建 Webhook' : '编辑 Webhook' }}</h3>
        <el-alert
          title="通知内容推送到指定URL, POST请求, 设置Header[ Content-Type: application/json]"
          type="info"
          :closable="false">
        </el-alert><br>
        <el-form-item label="名称" prop="name">
          <el-input v-model.trim="form.name" maxlength="64" placeholder="例如：运维钉钉群、日报飞书群"></el-input>
        </el-form-item>
        <el-form-item label="平台">
          <el-select v-model="form.provider" @change="changeProvider">
            <el-option v-for="item in providers" :key="item.value" :label="item.label" :value="item.value"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="URL" prop="url">
          <el-input v-model.trim="form.url" placeholder="填写完整的机器人 Webhook URL" autocomplete="off"></el-input>
        </el-form-item>
        <template v-if="form.provider !== 'generic'">
          <el-form-item label="加签">
            <el-switch v-model="form.sign_enabled"></el-switch>
            <div class="webhook-hint">机器人安全设置启用了加签时，请打开此选项。</div>
          </el-form-item>
          <el-form-item label="签名密钥">
            <el-input v-model.trim="form.secret" type="password" show-password autocomplete="new-password"
              :disabled="form.clear_secret"
              :placeholder="hasUsableSecret ? '已配置密钥，留空保留原密钥' : (form.provider === 'dingtalk' ? '填写钉钉 SEC 开头的签名密钥' : '填写飞书机器人签名密钥')">
            </el-input>
            <div class="webhook-hint">密钥保存后不回显；修改 URL 或模板时，留空会保留原密钥。切换平台需重新填写。</div>
            <el-checkbox v-if="hasUsableSecret" v-model="form.clear_secret">清除已保存的密钥（需关闭加签）</el-checkbox>
          </el-form-item>
        </template>
        <el-form-item label="模板" prop="template">
          <el-input
            type="textarea"
            :rows="8"
            placeholder=""
            v-model.trim="form.template">
          </el-input>
          <el-button type="text" @click="useDefaultTemplate">填入当前平台默认模板</el-button>
          <div class="webhook-hint">可以自定义消息 JSON；时间戳和签名由后端自动生成，不要手动填入。</div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" :disabled="testing" @click="submit()">保存</el-button>
          <el-button :loading="testing && testingId === null" :disabled="saving || testing" @click="testCurrent">测试发送</el-button>
          <el-button :disabled="saving || testing" @click="editing = false">取消</el-button>
        </el-form-item>
      </el-form>
    </el-main>
  </el-container>
</template>

<script>
import systemSidebar from '../sidebar'
import notificationTab from './tab'
import notificationService from '../../../api/notification'
import {WEBHOOK_PROVIDERS, WEBHOOK_TEMPLATES, webhookPayload} from '../../../utils/webhook'
export default {
  name: 'notification-webhook',
  data () {
    return {
      form: {
        id: -1,
        name: '',
        url: '',
        template: '',
        provider: 'generic',
        sign_enabled: false,
        secret: '',
        clear_secret: false
      },
      endpoints: [],
      editing: false,
      testing: false,
      testingId: null,
      providers: WEBHOOK_PROVIDERS,
      storedProvider: 'generic',
      previousProvider: 'generic',
      hasSecret: false,
      saving: false,
      formRules: {
        name: [{required: true, message: '请输入 Webhook 名称', trigger: 'blur'}],
        url: [
          {type: 'url', required: true, message: '请输入有效的通知URL', trigger: 'blur'}
        ],
        template: [
          {required: true, message: '请输入通知模板', trigger: 'blur'}
        ]
      }
    }
  },
  components: {notificationTab, systemSidebar},
  computed: {
    hasUsableSecret () { return this.hasSecret && this.form.provider === this.storedProvider }
  },
  created () {
    this.init()
  },
  methods: {
    changeProvider (provider) {
      const defaultTemplate = WEBHOOK_TEMPLATES[this.previousProvider]
      if (!this.form.template || this.form.template.trim() === defaultTemplate.trim()) {
        this.form.template = WEBHOOK_TEMPLATES[provider]
      }
      this.previousProvider = provider
      this.form.sign_enabled = false
      this.form.secret = ''
      this.form.clear_secret = false
    },
    useDefaultTemplate () {
      this.$appConfirm(() => { this.form.template = WEBHOOK_TEMPLATES[this.form.provider] })
    },
    submit () {
      if (this.saving) return
      this.$refs['form'].validate((valid) => {
        if (!valid) {
          return false
        }
        this.save()
      })
    },
    save () {
      let payload
      try { payload = webhookPayload(this.form, this.hasUsableSecret) } catch (error) { this.$message.error(error.message); return }
      this.saving = true
      notificationService.storeWebhook(payload, () => {
        this.saving = false
        this.$message.success('Webhook 已保存')
        this.editing = false
        this.form.secret = ''
        this.init()
      }, () => { this.saving = false })
    },
    init () {
      notificationService.webhooks((data) => {
        this.endpoints = data || []
      })
    },
    providerLabel (provider) {
      const item = this.providers.find(item => item.value === provider)
      return item ? item.label : '通用 Webhook'
    },
    createWebhook () {
      this.form = {id: -1,
        name: '',
        url: '',
        template: WEBHOOK_TEMPLATES.generic,
        provider: 'generic',
        sign_enabled: false,
        secret: '',
        clear_secret: false}
      this.hasSecret = false
      this.storedProvider = 'generic'
      this.previousProvider = 'generic'
      this.editing = true
    },
    editWebhook (data) {
      this.form = {id: data.id,
        name: data.name,
        url: data.url,
        template: data.template,
        provider: data.provider || 'generic',
        sign_enabled: Boolean(data.sign_enabled),
        secret: '',
        clear_secret: false}
      this.storedProvider = this.form.provider
      this.previousProvider = this.form.provider
      this.hasSecret = Boolean(data.has_secret)
      this.editing = true
    },
    removeWebhook (endpoint) {
      this.$confirm(`删除「${endpoint.name}」？正在被任务使用的 Webhook 需先解除关联。`, '删除 Webhook',
        {type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消'}).then(() => {
        notificationService.removeWebhook(endpoint.id, () => {
          if (this.editing && this.form.id === endpoint.id) this.editing = false
          this.$message.success('Webhook 已删除')
          this.init()
        })
      }).catch(() => {})
    },
    testSaved (endpoint) {
      this.confirmTest({id: endpoint.id}, endpoint.name, endpoint.id)
    },
    testCurrent () {
      if (this.saving || this.testing) return
      this.$refs.form.validate(valid => {
        if (!valid) return
        let payload
        try { payload = webhookPayload(this.form, this.hasUsableSecret) } catch (error) { this.$message.error(error.message); return }
        this.confirmTest(payload, this.form.name, null)
      })
    },
    confirmTest (payload, name, id) {
      if (this.testing) return
      this.$confirm(`向「${name || '当前 Webhook'}」发送一条真实测试通知？此操作不会保存尚未保存的修改。`, '测试发送',
        {type: 'warning', confirmButtonText: '发送测试通知', cancelButtonText: '取消'}).then(() => {
        if (this.testing) return
        this.testing = true
        this.testingId = id
        const done = () => { this.testing = false; this.testingId = null }
        notificationService.testWebhook(payload, () => {
          done()
          this.$message.success('测试通知已发送，请查看机器人消息')
        }, done)
      }).catch(() => {})
    }
  }
}
</script>

<style scoped>
.webhook-hint { color: #909399; font-size: 13px; line-height: 1.6; margin-top: 6px; }
.webhook-toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 16px; margin-bottom: 18px; }
.webhook-form { max-width: 760px; margin-top: 24px; }
.webhook-delete { color: #f56c6c; }
</style>
