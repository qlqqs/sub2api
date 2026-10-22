<template>
  <AdminAffiliateRecordsTable :key="initialSearch" type="rebates" :initial-search="initialSearch" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AdminAffiliateRecordsTable from './AdminAffiliateRecordsTable.vue'

// The agent management page links here with ?search=<email> to filter by user.
const route = useRoute()
const initialSearch = computed(() => (typeof route.query.search === 'string' ? route.query.search : ''))
</script>
