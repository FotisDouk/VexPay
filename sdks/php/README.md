# VexPay PHP SDK

Official PHP client for [VexPay](../../) — self-hosted, non-custodial crypto payments.

Zero runtime dependencies beyond `ext-curl` and `ext-json`, so it runs on any typical PHP host.
Requires PHP 8.1+.

## Install

```sh
composer require vexpay/sdk
```

## Create an invoice

```php
$vexpay = new \VexPay\Client('https://pay.example.com', 'vpk_live_...');

$invoice = $vexpay->createInvoice([
    'chain'  => 'bitcoin',
    'asset'  => 'BTC',
    'amount' => '0.005',
    'wallet' => ['xpub' => 'zpub...'],
]);

echo $invoice->paymentUri;      // bitcoin:bc1q...?amount=0.00500000
echo $invoice->receiveAddress;  // bc1q...
```

Price in fiat and let VexPay lock the rate:

```php
$invoice = $vexpay->createInvoice([
    'chain'         => 'bitcoin',
    'fiat_currency' => 'EUR',
    'fiat_amount'   => '25',
    'wallet'        => ['xpub' => 'zpub...'],
]);
```

Fetch or list invoices, and get a QR code:

```php
$invoice = $vexpay->getInvoice('inv_...');
$page    = $vexpay->listInvoices(limit: 20);
$png     = $vexpay->getInvoiceQrPng('inv_...', size: 320); // raw PNG bytes
```

## Verify webhooks

```php
$payload = file_get_contents('php://input');
$header  = $_SERVER['HTTP_VEXPAY_SIGNATURE'] ?? '';

try {
    $event = \VexPay\Webhook::constructEvent($payload, $header, $webhookSecret);
} catch (\VexPay\Exception\SignatureVerificationException $e) {
    http_response_code(400);
    exit;
}

if ($event->invoice->isPaid()) {
    // fulfil the order in $event->invoice->metadata['order_id']
}
```

> Amounts (`amount`, `received`, `rate`) are exact decimal strings. Never cast them to `float`
> for accounting or comparisons.

## Examples

- [`examples/create_invoice.php`](examples/create_invoice.php)
- [`examples/webhook_endpoint.php`](examples/webhook_endpoint.php)

## Tests

```sh
composer install
composer test
```

## License

AGPL-3.0-or-later.
