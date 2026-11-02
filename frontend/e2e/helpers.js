const API_BASE = process.env.API_BASE_URL || 'http://localhost:8080/api/v1';

// mintDevToken hits the same dev-token shortcut used in every Postman
// demo this project has had — no tenant_id in the body means a fresh
// tenant, exactly like two different Postman users.
export async function mintDevToken(request, role = 'manager') {
  const res = await request.post(`${API_BASE}/auth/dev-token`, { data: { role } });
  return res.json(); // { token, tenant_id }
}

export async function createVehicle(request, token, overrides = {}) {
  const res = await request.post(`${API_BASE}/vehicles`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { plate_number: 'E2E0001', model: 'Playwright Transit', ...overrides },
  });
  return res.json();
}

export async function createDelivery(request, token, overrides = {}) {
  const res = await request.post(`${API_BASE}/deliveries`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { pickup_address: 'Depot', dropoff_address: 'Client St', ...overrides },
  });
  return res.json();
}

// signInAs skips the login form and plants a session directly — for
// specs whose point is the data on the page, not the sign-in flow
// itself (that's auth.spec.js's job).
export async function signInAs(page, token, tenantId) {
  await page.addInitScript(
    ([t, tid]) => {
      window.localStorage.setItem('token', t);
      window.localStorage.setItem('tenant_id', tid);
    },
    [token, tenantId],
  );
}