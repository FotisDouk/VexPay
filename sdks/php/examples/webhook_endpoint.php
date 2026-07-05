<?php

declare(strict_types=1);

// A minimal webhook receiver. Point VEXPAY_WEBHOOK_URL at this script and set
// VEXPAY_WEBHOOK_SECRET to the same value used here.

require __DIR__ . '/../vendor/autoload.php';

use VexPay\Webhook;
use VexPay\Exception\SignatureVerificationException;

$secret = getenv('VEXPAY_WEBHOOK_SECRET') ?: 'whsec_replace_me';

$payload = file_get_contents('php://input') ?: '';
$header = $_SERVER['HTTP_VEXPAY_SIGNATURE'] ?? '';

try {
    $event = Webhook::constructEvent($payload, $header, $secret);
} catch (SignatureVerificationException $e) {
    http_response_code(400);
    echo 'invalid signature';
    exit;
}

// Signature verified — safe to act on the event.
if ($event->type === 'invoice.paid' || $event->type === 'invoice.overpaid') {
    $invoice = $event->invoice;
    // TODO: mark order $invoice->metadata['order_id'] as paid, fulfil, etc.
    error_log(sprintf('Invoice %s is %s', $invoice->id, $invoice->status));
}

// Acknowledge quickly so VexPay stops retrying.
http_response_code(200);
echo 'ok';
