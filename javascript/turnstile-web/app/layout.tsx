import type { Metadata } from "next";

import { AuthProvider } from "@/lib/auth-context";
import "./globals.css";

export const metadata: Metadata = {
  title: "Turnstile",
  description: "The session layer for the autonomous AI era.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="h-full">
      <body className="min-h-full">
        <div
          className="orb"
          style={{ width: 360, height: 360, background: "#6366f1", top: -80, left: -60 }}
        />
        <div
          className="orb"
          style={{
            width: 420,
            height: 420,
            background: "#ec4899",
            bottom: -120,
            right: -80,
            animationDelay: "5s",
          }}
        />
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
