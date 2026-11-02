import { test, expect } from '@playwright/test';
import { mintDevToken, createDelivery, signInAs } from './helpers';

test('a delivery created via the API appears with its status', async ({ page, request }) => {
  const { token, tenant_id } = await mintDevToken(request);
  await createDelivery(request, token, { pickup_address: 'Warehouse 7', dropoff_address: 'Client Ave 12' });

  await signInAs(page, token, tenant_id);
  await page.goto('/deliveries');

  await expect(page.getByText('Warehouse 7')).toBeVisible();
  await expect(page.getByText('Client Ave 12')).toBeVisible();
  await expect(page.getByText('pending').first()).toBeVisible();
});

test('empty state tells the user what to do, not just a blank table', async ({ page, request }) => {
  const { token, tenant_id } = await mintDevToken(request);
  await signInAs(page, token, tenant_id);
  await page.goto('/deliveries');

  await expect(page.getByText(/No deliveries yet/)).toBeVisible();
});