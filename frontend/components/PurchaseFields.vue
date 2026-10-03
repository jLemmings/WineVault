<script setup>
import { currencies } from '~/utils/purchases'
defineProps({draft:Object})
</script>
<template><div class="purchase-fields"><h3>Purchase details <small>Optional · price per bottle</small></h3><div class="form-row"><label>Price per bottle<input v-model="draft.price" type="number" min="0" max="10000000000" step="0.01" placeholder="Unknown"></label><label>Currency<select v-model="draft.currency" aria-label="Currency"><option v-for="currency in currencies" :key="currency">{{currency}}</option></select></label></div><label>Purchase date<input v-model="draft.purchaseDate" type="date" min="1900-01-01" max="9999-12-31"></label><label>Seller<input v-model="draft.seller" maxlength="200" placeholder="Shop, winery, or website"></label></div></template>
<style scoped>.purchase-fields{text-align:left;margin:20px 0}.purchase-fields h3{font-size:15px;margin-bottom:12px}.purchase-fields h3 small{display:block;font-size:11px;font-weight:400;color:#857b70;margin-top:4px}.form-row{min-width:0}.form-row label{min-width:0}</style>
