<template>
  <div class="workspace">
    <section class="panel">
      <div class="panel-title">
        上传规则
      </div>
      <p class="lede">
        控制上传并发、文件类型、目录边界和存储配额。当前上传入口在文件管理页。
      </p>
      <div class="rule-grid">
        <el-tag>单文件上传</el-tag>
        <el-tag type="success">
          512 MB 默认上限
        </el-tag>
        <el-tag type="warning">
          后端路径校验
        </el-tag>
      </div>
    </section>
  </div>
</template>
