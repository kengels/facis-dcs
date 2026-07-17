import { useJwt } from '@vueuse/integrations/useJwt'
import { defineStore } from 'pinia'
import { computed, type Ref, ref } from 'vue'
import { mapRoleLabelsToUserRoles, rolesFromJwtPayload, type UserRole } from '@/types/user-role'
import { useAuthTokenStore } from './auth-token-store'

interface User {
  issuer: string
  holder: string
  roles: UserRole[]
}

export const useAuthStore = defineStore('auth', () => {
  const authTokenStore = useAuthTokenStore()
  const user: Ref<User | null> = ref(null)

  const isAuthenticated = computed(() => !!user.value && authTokenStore.isAuthSet)

  function setHolder(holder: string): boolean {
    const authTokenStore = useAuthTokenStore()
    const payload = useJwt<{
      sub?: string
      exp?: number
      roles?: unknown
      ext?: { iss?: string; roles?: unknown }
    }>(authTokenStore.accessToken).payload.value

    if (payload?.sub !== holder) {
      console.error('User Error: JWT sub mismatch', { expected: holder, sub: payload?.sub })
      return false
    }

    if (typeof payload.exp !== 'number' || payload.exp * 1000 <= Date.now()) {
      return false
    }

    const roles = mapRoleLabelsToUserRoles(rolesFromJwtPayload(payload))
    if (roles.length === 0) {
      console.error('User Error: Hydra access token has no mapped roles', { sub: payload.sub })
      return false
    }

    user.value = {
      holder: holder,
      issuer: payload?.ext?.iss ?? '',
      roles,
    }
    return true
  }

  function restoreFromToken(): boolean {
    const holder = authTokenStore.getHolder
    if (!authTokenStore.isAuthSet || typeof holder !== 'string' || holder.length === 0) {
      return false
    }
    return setHolder(holder)
  }

  function remove() {
    user.value = null
  }

  return { user, isAuthenticated, setHolder, restoreFromToken, remove }
})
