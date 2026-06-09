import { chromium } from "@playwright/test";

const base = process.env.E2E_BASE_URL ?? "http://localhost:3000";
const email = process.env.SEED_EMAIL;
const password = process.env.SEED_PASSWORD ?? "password123";

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1320, height: 900 } });

await page.goto(`${base}/login`);
await page.waitForLoadState("networkidle");
await page.screenshot({ path: "/tmp/shot-login.png" });

// Sign in with the seeded account (which already has an org + telemetry)
await page.getByPlaceholder("you@company.com").fill(email);
await page.getByPlaceholder("••••••••").fill(password);
await page.getByRole("button", { name: "Sign in" }).click();
await page.waitForURL("**/dashboard");
await page.waitForLoadState("networkidle");
await page.screenshot({ path: "/tmp/shot-dashboard.png" });

// Open the seeded org → populated hero + sessions
await page.getByText("Northwind AI").first().click();
await page.waitForURL("**/organizations/**");
await page.waitForLoadState("networkidle");
await page.waitForTimeout(400);
await page.screenshot({ path: "/tmp/shot-org.png", fullPage: true });

await browser.close();
console.log("screenshots written to /tmp/shot-*.png");
