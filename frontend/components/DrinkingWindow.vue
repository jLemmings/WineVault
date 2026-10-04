<script setup>
  const props = defineProps({ bottle: Object }),
    emit = defineEmits(['saved', 'busy']);
  const start = ref(''),
    end = ref(''),
    busy = ref(false),
    error = ref('');
  watch(
    () => props.bottle,
    (b) => {
      start.value = b.drinkStart ?? '';
      end.value = b.drinkEnd ?? '';
      error.value = '';
    },
    { immediate: true },
  );
  async function save() {
    busy.value = true;
    emit('busy', true);
    error.value = '';
    try {
      await $fetch(`/api/bottles/${props.bottle.id}/window`, {
        method: 'PUT',
        body: {
          start: start.value === '' ? null : Number(start.value),
          end: end.value === '' ? null : Number(end.value),
          revision: props.bottle.revision,
        },
        retry: 0,
      });
      emit('saved');
    } catch (e) {
      error.value = typeof e.data === 'string' ? e.data : 'Could not save drinking window.';
    } finally {
      busy.value = false;
      emit('busy', false);
    }
  }
</script>
<template>
  <form class="drinking-window" @submit.prevent="save">
    <fieldset :disabled="busy">
      <legend>Your drinking window</legend>
      <p>
        Your chosen years apply to all bottles of this wine and vintage. Clear both to remove the
        window.
      </p>
      <div class="form-row">
        <label
          >Start year<input
            v-model="start"
            type="number"
            min="1900"
            max="9999"
            :required="end !== ''" /></label
        ><label
          >End year<input
            v-model="end"
            type="number"
            :min="start || 1900"
            max="9999"
            :required="start !== ''"
        /></label>
      </div>
      <p v-if="error" role="alert" class="scanner-error">{{ error }}</p>
      <button class="editor-secondary">{{ busy ? 'Saving…' : 'Save window' }}</button>
    </fieldset>
  </form>
</template>
<style scoped>
  .drinking-window {
    text-align: left;
    margin: 20px 0;
  }
  fieldset {
    border: 0;
    padding: 0;
    min-width: 0;
  }
  legend {
    font-weight: 600;
  }
  p {
    font-size: 12px;
    line-height: 1.6;
    margin: 10px 0;
    color: #857b70;
  }
</style>
