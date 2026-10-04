<script setup>
  import { Bell, Settings2, LogOut } from 'lucide-vue-next';

  defineProps({
    initials: { type: String, default: '' },
    owner: { type: String, required: true },
    loaded: { type: Boolean, default: false },
    notificationCount: { type: Number, default: 0 },
  });
  const emit = defineEmits(['settings', 'notifications', 'sign-out']);
  const open = ref(false);
  const root = ref(null);
  const trigger = ref(null);
  const panel = ref(null);

  function close(restoreFocus = false) {
    open.value = false;
    if (restoreFocus) trigger.value?.focus();
  }
  function select(action) {
    close(true);
    emit(action);
  }
  function outsideClick(event) {
    if (!root.value?.contains(event.target)) close();
  }
  function focusOutside(event) {
    if (!root.value?.contains(event.target)) close();
  }
  function keydown(event) {
    if (event.key === 'Escape' && open.value) {
      event.preventDefault();
      event.stopPropagation();
      close(true);
    }
  }
  async function focusFirst() {
    open.value = true;
    await nextTick();
    panel.value?.querySelector('button:not(:disabled)')?.focus();
  }
  onMounted(() => {
    document.addEventListener('click', outsideClick);
    document.addEventListener('focusin', focusOutside);
  });
  onBeforeUnmount(() => {
    document.removeEventListener('click', outsideClick);
    document.removeEventListener('focusin', focusOutside);
  });
</script>

<template>
  <div ref="root" class="profile-menu" @keydown="keydown">
    <button
      ref="trigger"
      class="avatar small profile-trigger"
      aria-label="My profile"
      :aria-expanded="open"
      aria-controls="profile-actions"
      @click="open = !open"
      @keydown.down.prevent="focusFirst"
    >
      {{ initials || 'P' }}
      <span v-if="notificationCount" class="profile-notification-dot" aria-hidden="true"></span>
    </button>
    <div
      v-if="open"
      id="profile-actions"
      ref="panel"
      class="profile-actions"
      aria-label="Profile actions"
    >
      <div class="profile-menu-owner">{{ owner }}</div>
      <button aria-label="Cellar settings" :disabled="!loaded" @click="select('settings')">
        <Settings2 :size="17" />Settings
      </button>
      <button aria-label="Notifications" @click="select('notifications')">
        <Bell :size="17" />Notifications
        <span v-if="notificationCount" class="notification-count">{{ notificationCount }}</span>
      </button>
      <button class="profile-sign-out" @click="select('sign-out')">
        <LogOut :size="17" />Sign out
      </button>
    </div>
  </div>
</template>

<style scoped>
  .profile-menu {
    position: relative;
  }
  .profile-trigger {
    padding: 0;
  }
  .profile-notification-dot {
    position: absolute;
    top: 0;
    right: 0;
    width: 7px;
    height: 7px;
    border: 2px solid #fff;
    border-radius: 50%;
    background: var(--wine);
  }
  .profile-actions {
    position: absolute;
    top: calc(100% + 12px);
    right: 0;
    width: 220px;
    max-width: calc(100vw - 32px);
    padding: 8px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: #fff;
    box-shadow: 0 8px 30px #302d2c18;
    z-index: 40;
  }
  .profile-menu-owner {
    padding: 9px 10px 12px;
    border-bottom: 1px solid var(--line);
    margin-bottom: 4px;
    font-weight: 600;
    overflow-wrap: anywhere;
  }
  .profile-actions button {
    display: flex;
    justify-content: flex-start;
    width: 100%;
    padding: 11px 10px;
    border-radius: 6px;
    text-align: left;
    font-size: 12px;
  }
  .profile-actions button:hover,
  .profile-actions button:focus-visible {
    background: var(--cream);
  }
  .profile-actions button:focus-visible {
    outline-offset: -2px;
  }
  .notification-count {
    margin-left: auto;
    border-radius: 12px;
    background: #f4e8ed;
    color: var(--wine);
    padding: 2px 7px;
    font-size: 10px;
  }
  .profile-actions .profile-sign-out {
    margin-top: 4px;
    border-top: 1px solid var(--line);
    border-radius: 0 0 6px 6px;
    color: var(--wine);
  }
</style>
