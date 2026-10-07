<template>
  <el-table :data="tasks" row-key="id" border style="width: 100%" @selection-change="$emit('selection-change', $event)">
    <el-table-column v-if="isAdmin" type="selection" width="45"></el-table-column>
    <el-table-column type="expand" width="45">
      <template slot-scope="scope">
        <el-form label-position="left" inline class="task-details">
          <el-form-item label="任务创建时间:">{{scope.row.created | formatTime}}</el-form-item>
          <el-form-item label="任务类型:">{{scope.row.level | formatLevel}}</el-form-item>
          <el-form-item label="单实例运行:">{{scope.row.multi | formatMulti}}</el-form-item>
          <el-form-item label="超时时间:">{{scope.row.timeout | formatTimeout}}</el-form-item>
          <el-form-item label="重试次数:">{{scope.row.retry_times}}</el-form-item>
          <el-form-item label="重试间隔:">{{scope.row.retry_interval | formatRetryTimesInterval}}</el-form-item>
          <el-form-item label="任务节点:" style="width: 100%">
            <div v-for="item in scope.row.hosts" :key="item.host_id">{{item.alias}} - {{item.name}}:{{item.port}}</div>
          </el-form-item>
          <el-form-item label="命令:" style="width: 100%">{{scope.row.command}}</el-form-item>
          <el-form-item label="备注:" style="width: 100%">{{scope.row.remark}}</el-form-item>
        </el-form>
      </template>
    </el-table-column>
    <el-table-column prop="id" label="任务ID" width="75"></el-table-column>
    <el-table-column prop="name" label="任务名称" min-width="150"></el-table-column>
    <el-table-column label="执行时间" min-width="200">
      <template slot-scope="scope"><span>{{scope.row.spec | formatSchedule}}</span></template>
    </el-table-column>
    <el-table-column label="下次执行时间" width="160">
      <template slot-scope="scope">{{scope.row.next_run_time | formatTime}}</template>
    </el-table-column>
    <el-table-column prop="protocol" :formatter="formatProtocol" label="执行方式" width="100"></el-table-column>
    <el-table-column label="状态" width="80">
      <template slot-scope="scope">
        <el-switch
          v-if="scope.row.level === 1"
          v-model="scope.row.status"
          :active-value="1"
          :inactive-value="0"
          :disabled="!isAdmin"
          active-color="#13ce66"
          inactive-color="#ff4949"
          @change="$emit('status', scope.row)">
        </el-switch>
      </template>
    </el-table-column>
    <el-table-column v-if="isAdmin" label="操作" width="220">
      <template slot-scope="scope">
        <el-row>
          <el-button type="primary" @click="$emit('edit', scope.row)">编辑</el-button>
          <el-button type="success" @click="$emit('run', scope.row)">手动执行</el-button>
        </el-row>
        <br>
        <el-row>
          <el-button type="info" @click="$emit('log', scope.row)">查看日志</el-button>
          <el-button type="danger" @click="$emit('remove', scope.row)">删除</el-button>
        </el-row>
      </template>
    </el-table-column>
  </el-table>
</template>

<script>
import {describeSchedule} from '../../utils/schedule'

export default {
  name: 'task-table',
  props: {
    tasks: {type: Array, default: () => []},
    isAdmin: {type: Boolean, default: false}
  },
  filters: {
    formatSchedule: describeSchedule,
    formatLevel: value => value === 1 ? '主任务' : '子任务',
    formatTimeout: value => value > 0 ? value + '秒' : '不限制',
    formatRetryTimesInterval: value => value > 0 ? value + '秒' : '系统默认',
    formatMulti: value => value > 0 ? '否' : '是'
  },
  methods: {
    formatProtocol (row) {
      return row.protocol === 2 ? 'shell' : row.http_method === 1 ? 'http-get' : 'http-post'
    }
  }
}
</script>

<style scoped>
.task-details { font-size: 0; }
.task-details .el-form-item { margin-right: 0; margin-bottom: 0; width: 50%; }
</style>
