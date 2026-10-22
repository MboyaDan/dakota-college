import { MessageCircle } from "@/components/icons";
import { whatsappHref } from "@/lib/public/settings";

/**
 * Floating WhatsApp click-to-chat (brief 5.1, 6.2) with a prefilled message
 * and the office hours. Hidden until the college sets a WhatsApp number.
 */
export function WhatsAppButton({ number, officeHours }: { number: string; officeHours: string }) {
  const href = whatsappHref(number, "Hello Dakota Training College, I have a question:");
  if (!href) return null;
  const hours = officeHours ? ` We reply during office hours: ${officeHours}.` : "";
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`Chat with us on WhatsApp (opens WhatsApp).${hours}`}
      title={officeHours ? `Office hours: ${officeHours}` : undefined}
      className="fixed right-4 bottom-4 z-40 inline-flex min-h-touch min-w-touch items-center justify-center gap-2 rounded-full bg-[#14532d] p-3 font-semibold text-white shadow-lg hover:bg-[#0f3f22] sm:px-4 print:hidden"
    >
      <MessageCircle aria-hidden className="size-6 sm:size-5" />
      {/* Icon only on phones, so it covers less of the page; the name is in aria-label. */}
      <span className="hidden sm:inline">WhatsApp</span>
    </a>
  );
}
