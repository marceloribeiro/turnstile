import { expect, test } from "@playwright/test";

// Happy-path: register → land on dashboard → create an organization → see it.
test("a new user can register and create an organization", async ({ page }) => {
  const email = `e2e-${Date.now()}@example.com`;

  await page.goto("/register");
  await page.getByPlaceholder("Ada").fill("E2E");
  await page.getByPlaceholder("Lovelace").fill("User");
  await page.getByPlaceholder("you@company.com").fill(email);
  await page.getByPlaceholder("At least 8 characters").fill("password123");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.getByRole("heading", { name: "Organizations" })).toBeVisible();

  await page.getByPlaceholder("Acme Inc.").fill("Northwind AI");
  await page.getByRole("button", { name: "Create organization" }).click();

  await expect(page.getByText("Northwind AI")).toBeVisible();

  // open the org → Overview tab with the dollars-prevented hero
  await page.getByText("Northwind AI").click();
  await expect(page).toHaveURL(/\/organizations\//);
  await expect(page.getByText("Dollars prevented")).toBeVisible();

  // tab nav: Members and Projects are their own pages
  await page.getByRole("link", { name: "Members" }).click();
  await expect(page).toHaveURL(/\/members$/);
  await expect(page.getByRole("heading", { name: "Members" })).toBeVisible();

  await page.getByRole("link", { name: "Projects" }).click();
  await expect(page).toHaveURL(/\/projects$/);
  await expect(page.getByRole("heading", { name: "New project" })).toBeVisible();

  // create a project and open its detail page
  await page.getByPlaceholder("Checkout agents").fill("Support bots");
  await page.getByRole("button", { name: "Create project" }).click();
  await page.getByText("Support bots").click();
  await expect(page).toHaveURL(/\/projects\/[^/]+$/);
  await expect(page.getByRole("heading", { name: "Ingest keys" })).toBeVisible();
});
