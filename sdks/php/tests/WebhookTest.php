<?php

declare(strict_types=1);

namespace VexPay\Tests;

use PHPUnit\Framework\TestCase;
use VexPay\Exception\SignatureVerificationException;
use VexPay\Webhook;

final class WebhookTest extends TestCase
{
    private function sign(string $payload, string $secret, int $t): string
    {
        $sig = hash_hmac('sha256', $t . '.' . $payload, $secret);
        return "t={$t},v1={$sig}";
    }

    public function testConstructEventReturnsParsedEvent(): void
    {
        $payload = '{"id":"evt_1","type":"invoice.paid","data":{"id":"inv_1","status":"paid","amount":"0.001"}}';
        $secret = 'whsec_test';
        $header = $this->sign($payload, $secret, time());

        $event = Webhook::constructEvent($payload, $header, $secret);

        self::assertSame('invoice.paid', $event->type);
        self::assertSame('inv_1', $event->invoice->id);
        self::assertTrue($event->invoice->isPaid());
    }

    public function testWrongSecretIsRejected(): void
    {
        $this->expectException(SignatureVerificationException::class);
        $payload = '{}';
        $header = $this->sign($payload, 'right-secret', time());
        Webhook::verifySignature($payload, $header, 'wrong-secret');
    }

    public function testTamperedBodyIsRejected(): void
    {
        $this->expectException(SignatureVerificationException::class);
        $secret = 'whsec_test';
        $header = $this->sign('{"amount":"1"}', $secret, time());
        Webhook::verifySignature('{"amount":"999"}', $header, $secret);
    }

    public function testStaleTimestampIsRejected(): void
    {
        $this->expectException(SignatureVerificationException::class);
        $payload = '{}';
        $header = $this->sign($payload, 'whsec_test', time() - 1000);
        Webhook::verifySignature($payload, $header, 'whsec_test', 300);
    }

    public function testMalformedHeaderIsRejected(): void
    {
        $this->expectException(SignatureVerificationException::class);
        Webhook::verifySignature('{}', 'not-a-valid-header', 'whsec_test');
    }
}
