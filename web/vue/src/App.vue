<template>
  <el-container class="app-layout">
    <el-header class="app-header" height="auto">
      <app-header></app-header>
      <app-nav-menu></app-nav-menu>
    </el-header>
    <el-main class="app-body">
      <div id="main-container" v-cloak>
        <router-view :key="$route.fullPath"/>
      </div>
    </el-main>
    <el-footer>
      <app-footer></app-footer>
    </el-footer>
  </el-container>
</template>

<script>
import installService from './api/install'
import appHeader from './components/common/header.vue'
import appNavMenu from './components/common/navMenu.vue'
import appFooter from './components/common/footer.vue'
import './styles/responsive.css'

export default {
  name: 'App',
  created () {
    installService.status((data) => {
      if (!data) {
        this.$router.push('/install')
      }
    })
  },
  components: {
    appHeader,
    appNavMenu,
    appFooter
  }
}
</script>
<style>
  [v-cloak] {
    display: none !important;
  }
  body {
    margin:0;
  }
  .el-header {
    padding:0;
    margin:0;
  }
  .el-container {
    padding:0;
    margin:0;
    width: 100%;
  }
  .el-main {
    padding:0;
    margin:0;
  }
  .el-aside .el-menu {
    height: 100%;
  }
</style>
