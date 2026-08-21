#!/usr/bin/env node
/* global Buffer, console, process */

import { createHmac } from 'node:crypto';

const secret = process.argv[2]?.trim().toUpperCase();
if (!secret || !/^[A-Z2-7]+=*$/.test(secret)) {
  console.error('usage: just fake-totp BASE32_SECRET');
  process.exit(2);
}

const key = decodeBase32(secret);
const counter = Math.floor(Date.now() / 30_000);
const message = Buffer.alloc(8);
message.writeBigUInt64BE(BigInt(counter));
const digest = createHmac('sha1', key).update(message).digest();
const offset = (digest.at(-1) ?? 0) & 0x0f;
const binary =
  (((digest[offset] ?? 0) & 0x7f) << 24) |
  ((digest[offset + 1] ?? 0) << 16) |
  ((digest[offset + 2] ?? 0) << 8) |
  (digest[offset + 3] ?? 0);
const code = String(binary % 1_000_000).padStart(6, '0');
const secondsRemaining = 30 - (Math.floor(Date.now() / 1_000) % 30);

console.log(`${code} (${secondsRemaining}s remaining)`);

function decodeBase32(value) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const character of value.replaceAll('=', '')) {
    const index = alphabet.indexOf(character);
    if (index < 0) {
      console.error('BASE32_SECRET contains an invalid character');
      process.exit(2);
    }
    bits += index.toString(2).padStart(5, '0');
  }
  const bytes = [];
  for (let index = 0; index + 8 <= bits.length; index += 8)
    bytes.push(Number.parseInt(bits.slice(index, index + 8), 2));
  return Buffer.from(bytes);
}
