import { createHmac } from "node:crypto";

// RFC 6238 codes, as an authenticator app makes them, for the two-step
// sign-in test. offsetSeconds picks a neighbouring 30-second step.
export function totp(base32Key: string, offsetSeconds = 0): string {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = "";
  for (const ch of base32Key.replace(/\s/g, "").toUpperCase()) {
    bits += alphabet.indexOf(ch).toString(2).padStart(5, "0");
  }
  const key = Buffer.from(bits.match(/.{8}/g)!.map((b) => parseInt(b, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor((Date.now() / 1000 + offsetSeconds) / 30)));
  const mac = createHmac("sha1", key).update(counter).digest();
  const off = mac[mac.length - 1]! & 0x0f;
  const n = (mac.readUInt32BE(off) & 0x7fffffff) % 1_000_000;
  return n.toString().padStart(6, "0");
}
