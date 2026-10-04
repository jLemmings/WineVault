<script setup>
  import { Wine } from 'lucide-vue-next';

  const props = defineProps({
    wine: { type: Object, required: true },
    name: { type: String, required: true },
    large: { type: Boolean, default: false },
  });
  const failed = ref(false);
  // Image fields are optional: the public API does not guarantee bottle photos.
  const source = computed(() => {
    const image = props.wine.image;
    const values = [
      props.wine.image_url,
      props.wine.bottle_image_url,
      props.wine.photo_url,
      typeof image === 'string' ? image : image?.url,
    ];
    for (const value of values) {
      if (typeof value !== 'string') continue;
      try {
        const url = new URL(value);
        if (url.protocol === 'https:' && !url.username && !url.password) return url.href;
      } catch {
        // Ignore malformed URLs and keep looking for a usable image.
      }
    }
    return '';
  });
  watch(source, () => {
    failed.value = false;
  });
</script>

<template>
  <div class="bottle-image" :class="{ large }">
    <a
      v-if="source && !failed"
      :href="source"
      target="_blank"
      rel="noopener noreferrer"
      :aria-label="`View bottle image for ${name}`"
    >
      <img
        :src="source"
        :alt="`Bottle of ${name}`"
        loading="lazy"
        decoding="async"
        referrerpolicy="no-referrer"
        @error="failed = true"
      />
    </a>
    <div v-else class="image-unavailable">
      <Wine :size="large ? 30 : 24" :stroke-width="1.4" aria-hidden="true" />
      <span>No image available</span>
    </div>
  </div>
</template>

<style scoped>
  .bottle-image {
    width: 64px;
    height: 112px;
    flex-shrink: 0;
    border-radius: 8px;
    background: #fffaf7;
    overflow: hidden;
  }
  .bottle-image.large {
    width: 96px;
    height: 168px;
  }
  .bottle-image a {
    display: block;
    width: 100%;
    height: 100%;
  }
  .bottle-image img {
    width: 100%;
    height: 100%;
    padding: 6px;
    object-fit: contain;
  }
  .bottle-image a:focus-visible {
    outline: 2px solid #be9a59;
    outline-offset: -2px;
  }
  .image-unavailable {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    height: 100%;
    padding: 6px;
    color: #9a6371;
    text-align: center;
  }
  .image-unavailable span {
    font-size: 9px;
    line-height: 1.4;
  }
</style>
