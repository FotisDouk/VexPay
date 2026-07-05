<?php

declare(strict_types=1);

// Create an invoice and print the payment details.
//
//   composer install
//   VEXPAY_URL=http://localhost:8080 VEXPAY_API_KEY=vpk_test_... php examples/create_invoice.php

require __DIR__ . '/../vendor/autoload.php';

use VexPay\Client;
use VexPay\Exception\ApiException;

$client = new Client(
    getenv('VEXPAY_URL') ?: 'http://localhost:8080',
    getenv('VEXPAY_API_KEY') ?: 'vpk_test_replace_me',
);

try {
    // Crypto-priced. For a fiat price, drop asset/amount and pass:
    //   'fiat_currency' => 'EUR', 'fiat_amount' => '25'
    $invoice = $client->createInvoice([
        'chain'    => 'mock', // use 'bitcoin' with a real 'wallet' => ['xpub' => 'zpub...']
        'asset'    => 'tBTC',
        'amount'   => '0.0025',
        'metadata' => ['order_id' => 'A-1001'],
    ]);
} catch (ApiException $e) {
    fwrite(STDERR, 'Failed to create invoice: ' . $e->getMessage() . "\n");
    exit(1);
}

printf("Invoice:      %s\n", $invoice->id);
printf("Status:       %s\n", $invoice->status);
printf("Pay to:       %s\n", $invoice->receiveAddress);
printf("Payment URI:  %s\n", $invoice->paymentUri);
printf("Amount:       %s %s\n", $invoice->amount, $invoice->asset);
