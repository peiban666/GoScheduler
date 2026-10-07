<template>
  <el-container >
    <task-sidebar></task-sidebar>
    <el-main>
      <el-form ref="form" :model="form" :rules="formRules" label-width="180px" class="task-edit-form">
        <el-input v-model="form.id" type="hidden"></el-input>
        <el-row>
          <el-col :span="12">
            <el-form-item label="任务名称" prop="name">
              <el-input v-model.trim="form.name"></el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="任务分组" prop="tag">
              <group-picker v-model="form.tag" :groups="taskGroups" @created="refreshTaskGroups"></group-picker>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row v-if="form.level === 1">
          <el-col>
            <el-alert
              title="主任务可以配置多个子任务, 当主任务执行完成后，自动执行子任务
任务类型新增后不能变更"
              type="info"
              :closable="false">
            </el-alert>
            <el-alert
              title="强依赖: 主任务执行成功，才会运行子任务
弱依赖: 无论主任务执行是否成功，都会运行子任务"
              type="info"
              :closable="false">
            </el-alert> <br>
          </el-col>
        </el-row>
        <el-row>
          <el-col :span="7">
            <el-form-item label="任务类型">
              <el-select v-model.trim="form.level" :disabled="form.id !== '' ">
                <el-option
                  v-for="item in levelList"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="7" v-if="form.level === 1">
            <el-form-item label="依赖关系">
              <el-select v-model.trim="form.dependency_status">
                <el-option
                  v-for="item in dependencyStatusList"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="10">
            <el-form-item label="子任务ID" v-if="form.level === 1">
              <el-input v-model.trim="form.dependency_task_id" placeholder="多个ID逗号分隔"></el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row v-if="form.level === 1">
          <el-col :span="20">
            <el-form-item label="执行时间" prop="spec">
              <schedule-picker v-model="form.spec"></schedule-picker>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row>
          <el-col :span="8">
            <el-form-item label="执行方式">
              <el-select v-model.trim="form.protocol">
                <el-option
                  v-for="item in protocolList"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8" v-if="form.protocol === 1 ">
            <el-form-item label="请求方法">
              <el-select key="http-method" v-model.trim="form.http_method">
                <el-option
                  v-for="item in httpMethods"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8" v-else>
            <el-form-item label="任务节点">
              <el-select key="shell" v-model="selectedHosts" filterable multiple placeholder="请选择">
                <el-option
                  v-for="item in hosts"
                  :key="item.id"
                  :label="item.alias + ' - ' + item.name"
                  :value="item.id">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row>
          <el-col :span="16">
            <el-form-item label="命令" prop="command">
              <el-input
                type="textarea"
                :rows="5"
                size="medium"
                width="100"
                :placeholder="commandPlaceholder"
                v-model="form.command">
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row  v-if="form.protocol === 1 && form.http_method === 2">
          <el-col :span="16">
            <el-form-item label="请求体" prop="request_body">
              <el-input
                type="textarea"
                :rows="5"
                size="medium"
                width="100"
                v-model="form.request_body">
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row>
          <el-col>
            <el-alert
              title="任务执行超时强制结束, 取值0-86400(秒), 默认0, 不限制"
              type="info"
              :closable="false">
            </el-alert>
            <el-alert
              title="单实例运行, 前次任务未执行完成，下次任务调度时间到了是否要执行, 即是否允许多进程执行同一任务"
              type="info"
              :closable="false">
            </el-alert> <br>
          </el-col>
        </el-row>
        <el-row>
          <el-col :span="12">
            <el-form-item label="任务超时时间" prop="timeout">
              <el-input v-model.number.trim="form.timeout"></el-input>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="单实例运行">
              <el-select v-model.trim="form.multi">
                <el-option
                  v-for="item in runStatusList"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row>
        <el-col :span="12">
          <el-form-item label="任务失败重试次数" prop="retry_times">
            <el-input v-model.number.trim="form.retry_times"
                      placeholder="0 - 10, 默认0，不重试"></el-input>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="任务失败重试间隔时间" prop="retry_interval">
            <el-input v-model.number.trim="form.retry_interval" placeholder="0 - 3600 (秒), 默认0，执行系统默认策略"></el-input>
          </el-form-item>
        </el-col>
        </el-row>
        <el-row>
          <el-col :span="8">
            <el-form-item label="任务通知">
              <el-select v-model.trim="form.notify_status">
                <el-option
                  v-for="item in notifyStatusList"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8" v-if="form.notify_status !== 1">
            <el-form-item label="通知类型">
              <el-select v-model.trim="form.notify_type">
                <el-option
                  v-for="item in notifyTypes"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                  >
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8"
                  v-if="form.notify_status !== 1 && form.notify_type === 2">
            <el-form-item label="接收用户">
              <el-select key="notify-mail" v-model="selectedMailNotifyIds" filterable multiple placeholder="请选择">
                <el-option
                  v-for="item in mailUsers"
                  :key="item.id"
                  :label="item.username"
                  :value="item.id">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>

          <el-col :span="8"
                  v-if="form.notify_status !== 1 && form.notify_type === 3">
            <el-form-item label="发送Channel">
              <el-select key="notify-slack" v-model="selectedSlackNotifyIds" filterable multiple placeholder="请选择">
                <el-option
                  v-for="item in slackChannels"
                  :key="item.id"
                  :label="item.name"
                  selected="true"
                  :value="item.id">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8" v-if="form.notify_status !== 1 && form.notify_type === 4">
            <el-form-item label="发送 Webhook">
              <el-select key="notify-webhook" v-model="selectedWebhookIds" filterable multiple placeholder="请选择 Webhook">
                <el-option v-for="item in webhooks" :key="item.id"
                  :label="item.name + ' · ' + webhookProviderLabel(item.provider)" :value="item.id"></el-option>
              </el-select>
              <div v-if="webhooks.length === 0" class="webhook-empty">
                请先在<router-link to="/system/notification/webhook">通知配置</router-link>中新建 Webhook
              </div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row v-if="form.notify_status === 4">
          <el-col :span="12">
            <el-form-item label="任务执行输出关键字" prop="notify_keyword">
              <el-input v-model.trim="form.notify_keyword" placeholder="任务执行输出中包含此关键字将触发通知"></el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row>
          <el-col :span="16">
            <el-form-item label="备注">
              <el-input
                type="textarea"
                :rows="3"
                size="medium"
                width="100"
                v-model="form.remark">
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item>
          <el-button type="primary" @click="submit">保存</el-button>
          <el-button @click="cancel">取消</el-button>
        </el-form-item>
      </el-form>
    </el-main>
  </el-container>
</template>

<script>
import taskSidebar from './sidebar'
import schedulePicker from '../../components/task/schedulePicker'
import groupPicker from '../../components/task/groupPicker'
import {normalizeGroupName} from '../../utils/taskGroups'
import taskService from '../../api/task'
import notificationService from '../../api/notification'
import {WEBHOOK_PROVIDERS} from '../../utils/webhook'

export default {
  name: 'task-edit',
  data () {
    return {
      form: {
        id: '',
        name: '',
        tag: '',
        level: 1,
        dependency_status: 1,
        dependency_task_id: '',
        spec: '0 30 7 * * *',
        protocol: 2,
        http_method: 1,
        command: '',
        request_body: '',
        host_id: '',
        timeout: 0,
        multi: 2,
        notify_status: 1,
        notify_type: 2,
        notify_receiver_id: '',
        notify_keyword: '',
        retry_times: 0,
        retry_interval: 0,
        remark: ''
      },
      formRules: {
        tag: [{
          validator (rule, value, callback) {
            try {
              normalizeGroupName(value)
              callback()
            } catch (error) {
              callback(error)
            }
          },
          trigger: 'change'
        }],
        name: [
          {required: true, message: '请输入任务名称', trigger: 'blur'}
        ],
        spec: [
          {required: true, message: '请配置有效的执行时间', trigger: 'change'}
        ],
        command: [
          {required: true, message: '请输入命令', trigger: 'blur'}
        ],
        timeout: [
          {type: 'number', required: true, message: '请输入有效的任务超时时间', trigger: 'blur'}
        ],
        retry_times: [
          {type: 'number', required: true, message: '请输入有效的任务执行失败重试次数', trigger: 'blur'}
        ],
        retry_interval: [
          {type: 'number', required: true, message: '请输入有效的任务执行失败，重试间隔时间', trigger: 'blur'}
        ],
        notify_keyword: [
          {required: true, message: '请输入要匹配的任务执行输出关键字', trigger: 'blur'}
        ]
      },
      httpMethods: [
        {
          value: 1,
          label: 'get'
        },
        {
          value: 2,
          label: 'post'
        }
      ],
      protocolList: [
        {
          value: 1,
          label: 'http'
        },
        {
          value: 2,
          label: 'shell'
        }
      ],
      levelList: [
        {
          value: 1,
          label: '主任务'
        },
        {
          value: 2,
          label: '子任务'
        }
      ],
      dependencyStatusList: [
        {
          value: 1,
          label: '强依赖'
        },
        {
          value: 2,
          label: '弱依赖'
        }
      ],
      runStatusList: [
        {
          value: 2,
          label: '是'
        },
        {
          value: 1,
          label: '否'
        }
      ],
      notifyStatusList: [
        {
          value: 1,
          label: '不通知'
        },
        {
          value: 2,
          label: '失败通知'
        },
        {
          value: 3,
          label: '总是通知'
        },
        {
          value: 4,
          label: '关键字匹配通知'
        }
      ],
      notifyTypes: [
        {
          value: 2,
          label: '邮件'
        },
        {
          value: 3,
          label: 'Slack'
        },
        {
          value: 4,
          label: 'WebHook'
        }
      ],
      hosts: [],
      taskGroups: [],
      mailUsers: [],
      slackChannels: [],
      webhooks: [],
      selectedHosts: [],
      selectedMailNotifyIds: [],
      selectedSlackNotifyIds: [],
      selectedWebhookIds: []
    }
  },
  computed: {
    commandPlaceholder () {
      if (this.form.protocol === 1) {
        return '请输入URL地址'
      }

      return '请输入shell命令'
    }
  },
  components: {taskSidebar, schedulePicker, groupPicker},
  created () {
    const id = this.$route.params.id
    if (!id && typeof this.$route.query.group === 'string') this.form.tag = this.$route.query.group

    taskService.detail(id, (taskData, hosts, groups) => {
      if (id && !taskData) {
        this.$message.error('数据不存在')
        this.cancel()
        return
      }
      this.hosts = hosts || []
      this.taskGroups = groups || []
      if (!taskData) {
        return
      }
      this.form.id = taskData.id
      this.form.name = taskData.name
      this.form.tag = taskData.tag || ''
      this.form.level = taskData.level
      if (taskData.dependency_status) {
        this.form.dependency_status = taskData.dependency_status
      }
      this.form.dependency_task_id = taskData.dependency_task_id
      this.form.spec = taskData.spec
      this.form.protocol = taskData.protocol
      if (taskData.http_method) {
        this.form.http_method = taskData.http_method
      }
      this.form.command = taskData.command
      this.form.request_body = taskData.request_body
      this.form.timeout = taskData.timeout
      this.form.multi = taskData.multi ? 1 : 2
      this.form.notify_keyword = taskData.notify_keyword
      this.form.notify_status = taskData.notify_status + 1
      this.form.notify_receiver_id = taskData.notify_receiver_id
      if (taskData.notify_type) {
        this.form.notify_type = taskData.notify_type + 1
      }
      this.form.retry_times = taskData.retry_times
      this.form.retry_interval = taskData.retry_interval
      this.form.remark = taskData.remark
      taskData.hosts = taskData.hosts || []
      if (this.form.protocol === 2) {
        taskData.hosts.forEach((v) => {
          this.selectedHosts.push(v.host_id)
        })
      }

      if (this.form.notify_status > 1) {
        const notifyReceiverIds = this.form.notify_receiver_id.split(',')
        if (this.form.notify_type === 2) {
          notifyReceiverIds.forEach((v) => {
            this.selectedMailNotifyIds.push(parseInt(v))
          })
        } else if (this.form.notify_type === 3) {
          notifyReceiverIds.forEach((v) => {
            this.selectedSlackNotifyIds.push(parseInt(v))
          })
        } else if (this.form.notify_type === 4) {
          this.selectedWebhookIds = this.form.notify_receiver_id ? notifyReceiverIds.map(Number) : [0]
        }
      }
    })

    notificationService.mail((data) => {
      this.mailUsers = data.mail_users
    })

    notificationService.slack((data) => {
      this.slackChannels = data.channels
    })
    notificationService.webhookOptions(data => { this.webhooks = data || [] })
  },
  methods: {
    webhookProviderLabel (provider) {
      const option = WEBHOOK_PROVIDERS.find(item => item.value === provider)
      return option ? option.label : '通用'
    },
    refreshTaskGroups () {
      taskService.groups({}, groups => { this.taskGroups = groups || [] })
    },
    submit () {
      this.$refs['form'].validate((valid) => {
        if (!valid) {
          return false
        }
        if (this.form.protocol === 2 && this.selectedHosts.length === 0) {
          this.$message.error('请选择任务节点')
          return false
        }
        if (this.form.notify_status > 1) {
          if (this.form.notify_type === 2 && this.selectedMailNotifyIds.length === 0) {
            this.$message.error('请选择邮件接收用户')
            return false
          }
          if (this.form.notify_type === 3 && this.selectedSlackNotifyIds.length === 0) {
            this.$message.error('请选择Slack Channel')
            return false
          }
          if (this.form.notify_type === 4 && this.selectedWebhookIds.length === 0) {
            this.$message.error('请选择至少一个 Webhook')
            return false
          }
        }

        this.save()
      })
    },
    save () {
      this.form.tag = normalizeGroupName(this.form.tag)
      if (this.form.protocol === 2 && this.selectedHosts.length > 0) {
        this.form.host_id = this.selectedHosts.join(',')
      }
      if (this.form.notify_status > 1 && this.form.notify_type === 2) {
        this.form.notify_receiver_id = this.selectedMailNotifyIds.join(',')
      }
      if (this.form.notify_status > 1 && this.form.notify_type === 3) {
        this.form.notify_receiver_id = this.selectedSlackNotifyIds.join(',')
      }
      if (this.form.notify_status > 1 && this.form.notify_type === 4) {
        this.form.notify_receiver_id = this.selectedWebhookIds.join(',')
      }
      taskService.update(this.form, () => {
        this.$router.push('/task')
      })
    },
    cancel () {
      this.$router.push('/task')
    }
  }
}
</script>
