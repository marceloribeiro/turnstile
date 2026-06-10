import type { Metadata } from "next";

import "./globals.css";

const title = "Turnstile — Kill the one rogue LLM session, not your whole app";
const description =
  "Turnstile is a transparent proxy that meters every AI session in real time and trips a circuit breaker the instant one session goes rogue — a runaway agent loop or a blown budget — without taking down the rest of your app. Drop-in, session-precise, real-time.";

export const metadata: Metadata = {
  metadataBase: new URL("https://turnstileguard.com"),
  title,
  description,
  keywords: [
    "LLM proxy",
    "AI cost control",
    "agent circuit breaker",
    "LLM observability",
    "AI budget enforcement",
    "OpenAI compatible proxy",
    "runaway agent",
    "AI gateway",
  ],
  openGraph: {
    title,
    description,
    url: "https://turnstileguard.com",
    siteName: "Turnstile",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title,
    description,
  },
  icons: {
    icon: [{ url: "/favicon.svg", type: "image/svg+xml" }],
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className="h-full">
      <body className="min-h-full">{children}</body>
    </html>
  );
}
