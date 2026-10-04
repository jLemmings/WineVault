<script setup>
  const props = defineProps({ bottle: Object }),
    emit = defineEmits(['saved', 'busy']);
  const open = ref(false),
    busy = ref(false),
    error = ref(''),
    draft = reactive({});
  function start() {
    Object.assign(draft, props.bottle);
    open.value = true;
    error.value = '';
  }
  watch(
    () => props.bottle.id,
    () => (open.value = false),
  );
  async function save() {
    busy.value = true;
    emit('busy', true);
    error.value = '';
    try {
      await $fetch(`/api/bottles/${props.bottle.id}`, {
        method: 'PUT',
        body: {
          name: draft.name,
          vintage: Number(draft.vintage),
          region: draft.region,
          type: draft.type,
          revision: draft.revision,
        },
        retry: 0,
      });
      open.value = false;
      emit('saved');
    } catch (e) {
      error.value = typeof e.data === 'string' ? e.data : 'Could not save corrections.';
    } finally {
      busy.value = false;
      emit('busy', false);
    }
  }
</script>
<template>
  <div class="bottle-editor">
    <button v-if="!open" class="editor-secondary" @click="start">Edit bottle details</button>
    <form v-else @submit.prevent="save">
      <fieldset :disabled="busy">
        <legend>Correct bottle details</legend>
        <label>Wine name<input v-model="draft.name" required maxlength="150" /></label
        ><label
          >Vintage (0 for NV)<input
            v-model="draft.vintage"
            type="number"
            min="0"
            :max="new Date().getFullYear() + 1"
            required /></label
        ><label>Region<input v-model="draft.region" required maxlength="200" /></label
        ><label
          >Type<select v-model="draft.type">
            <option
              v-for="type in ['Red', 'White', 'Rosé', 'Champagne', 'Sparkling', 'Dessert']"
              :key="type"
            >
              {{ type }}
            </option>
          </select></label
        >
        <p v-if="error" role="alert" class="scanner-error">{{ error }}</p>
        <button type="button" class="editor-secondary" @click="open = false">Cancel</button
        ><button class="primary">{{ busy ? 'Saving…' : 'Save corrections' }}</button>
      </fieldset>
    </form>
  </div>
</template>
<style scoped>
  .bottle-editor {
    margin: 18px 0;
    text-align: left;
  }
  fieldset {
    border: 0;
    padding: 0;
    min-width: 0;
  }
  legend {
    font-weight: 600;
    margin-bottom: 12px;
  }
</style>
