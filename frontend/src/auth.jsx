import { createContext, useContext, useState } from 'react';

const AuthContext = createContext(null);

// Plain localStorage reads don't trigger a React re-render when they
// change — that was the bug: App computed `signedIn` once on first
// render, before sign-in, and never recomputed it after Login wrote the
// token. This context makes sign-in/out real React state, so anything
// that reads it (the nav bar) re-renders when it actually changes.
export function AuthProvider({ children }) {
  const [token, setToken] = useState(() => localStorage.getItem('token'));
  const [tenantId, setTenantId] = useState(() => localStorage.getItem('tenant_id'));

  function signIn(newToken, newTenantId) {
    localStorage.setItem('token', newToken);
    localStorage.setItem('tenant_id', newTenantId);
    setToken(newToken);
    setTenantId(newTenantId);
  }

  function signOut() {
    localStorage.removeItem('token');
    localStorage.removeItem('tenant_id');
    setToken(null);
    setTenantId(null);
  }

  return (
    <AuthContext.Provider value={{ token, tenantId, signedIn: !!token, signIn, signOut }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}