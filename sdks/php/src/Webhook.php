<?php

declare(strict_types=1);

namespace VexPay;

use VexPay\Exception\SignatureVerificationException;

/**
 * Verifies and parses VexPay webhook deliveries.
 *
 * The signature header has the form `t=<unix>,v1=<hex>`, where the HMAC-SHA256
 * is computed over `"<t>." . $payload` using your webhook secret.
 */
final class Webhook
{
    public const SIGNATURE_HEADER = 'VexPay-Signature';

    /**
     * Verify a webhook signature and return the parsed event.
     *
     * @param string $payload   The exact raw request body.
     * @param string $header    The value of the VexPay-Signature header.
     * @param string $secret    Your webhook signing secret.
     * @param int    $tolerance Max age of the signature in seconds (0 disables).
     *
     * @throws SignatureVerificationException
     */
    public static function constructEvent(
        string $payload,
        string $header,
        string $secret,
        int $tolerance = 300,
    ): Event {
        self::verifySignature($payload, $header, $secret, $tolerance);

        $decoded = json_decode($payload, true);
        if (!is_array($decoded)) {
            throw new SignatureVerificationException('Webhook payload is not valid JSON');
        }
        return Event::fromArray($decoded);
    }

    /**
     * Verify a signature without parsing the event.
     *
     * @throws SignatureVerificationException
     */
    public static function verifySignature(
        string $payload,
        string $header,
        string $secret,
        int $tolerance = 300,
    ): void {
        [$timestamp, $signature] = self::parseHeader($header);

        if ($tolerance > 0 && abs(time() - $timestamp) > $tolerance) {
            throw new SignatureVerificationException('Signature timestamp is outside the tolerance');
        }

        $expected = hash_hmac('sha256', $timestamp . '.' . $payload, $secret);
        if (!hash_equals($expected, $signature)) {
            throw new SignatureVerificationException('Signature verification failed');
        }
    }

    /**
     * @return array{0:int,1:string} [timestamp, signature]
     */
    private static function parseHeader(string $header): array
    {
        $timestamp = null;
        $signature = null;
        foreach (explode(',', $header) as $part) {
            $pair = explode('=', trim($part), 2);
            if (count($pair) !== 2) {
                continue;
            }
            [$key, $value] = $pair;
            if ($key === 't') {
                $timestamp = $value;
            } elseif ($key === 'v1') {
                $signature = $value;
            }
        }

        if ($timestamp === null || !ctype_digit($timestamp) || $signature === null) {
            throw new SignatureVerificationException('Malformed signature header');
        }
        return [(int) $timestamp, $signature];
    }
}
