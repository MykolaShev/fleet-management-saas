import { test, expect } from '@playwright/test';
import { mintDevToken, createVehicle, signInAs } from './helpers';

test('a vehicle created via the API appears in the Vehicles table', async ({ page, request }) => {
  const { token, tenant_id } = await mintDevToken(request);
  await createVehicle(request, token, { plate_number: 'E2E-AA01', model: 'E2E Transit' });

  await signInAs(page, token, tenant_id);
  await page.goto('/vehicles');

  await expect(page.getByText('E2E-AA01')).toBeVisible();
  await expect(page.getByText('E2E Transit')).toBeVisible();
  await expect(page.getByText('idle').first()).toBeVisible();
});

test('a different tenant never sees another tenant\'s vehicles', async ({ page, request }) => {
  const owner = await mintDevToken(request);
  await createVehicle(request, owner.token, { plate_number: 'PRIVATE01' });

  const other = await mintDevToken(request); // fresh tenant, no shared data
  await signInAs(page, other.token, other.tenant_id);
  await page.goto('/vehicles');

  await expect(page.getByText('PRIVATE01')).not.toBeVisible();
  await expect(page.getByText(/No vehicles yet/)).toBeVisible();
});