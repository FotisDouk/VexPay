<?php

declare(strict_types=1);

namespace VexPay;

use VexPay\Exception\ApiException;

/**
 * VexPay API client.
 *
 * Zero runtime dependencies beyond ext-curl and ext-json, so it runs on any
 * typical PHP host.
 *
 * ```php
 * $vexpay  = new \VexPay\Client('https://pay.example.com', 'vpk_live_...');
 * $invoice = $vexpay->createInvoice([
 *     'chain'  => 'bitcoin',
 *     'asset'  => 'BTC',
 *     'amount' => '0.005',
 *     'wallet' => ['xpub' => 'zpub...'],
 * ]);
 * echo $invoice->paymentUri;
 * ```
 */
final class Client
{
    private string $baseUrl;
    private int $timeout;

    public function __construct(string $baseUrl, private string $apiKey, array $options = [])
    {
        $this->baseUrl = rtrim($baseUrl, '/');
        $this->timeout = (int) ($options['timeout'] ?? 30);
    }

    /**
     * Create an invoice. See the API reference for accepted fields (crypto- or
     * fiat-priced).
     *
     * @param array<string,mixed> $params
     */
    public function createInvoice(array $params): Invoice
    {
        return Invoice::fromArray($this->request('POST', '/v1/invoices', $params));
    }

    public function getInvoice(string $id): Invoice
    {
        return Invoice::fromArray($this->request('GET', '/v1/invoices/' . rawurlencode($id)));
    }

    /**
     * @return Invoice[]
     */
    public function listInvoices(int $limit = 20, int $offset = 0): array
    {
        $query = http_build_query(['limit' => $limit, 'offset' => $offset]);
        $body = $this->request('GET', '/v1/invoices?' . $query);
        return array_map([Invoice::class, 'fromArray'], $body['data'] ?? []);
    }

    /**
     * Fetch the invoice's payment QR code as raw PNG bytes.
     */
    public function getInvoiceQrPng(string $id, int $size = 256): string
    {
        return $this->requestRaw('GET', '/v1/invoices/' . rawurlencode($id) . '/qr?size=' . $size);
    }

    /**
     * Simulate a payment against a sandbox invoice (sandbox API keys only).
     *
     * @param array<string,mixed> $params
     */
    public function sandboxPay(string $id, array $params = []): Invoice
    {
        return Invoice::fromArray(
            $this->request('POST', '/v1/sandbox/pay/' . rawurlencode($id), $params)
        );
    }

    /**
     * Perform a request and decode a JSON object response.
     *
     * @param array<string,mixed>|null $body
     * @return array<string,mixed>
     */
    private function request(string $method, string $path, ?array $body = null): array
    {
        $raw = $this->send($method, $path, $body, 'application/json');
        if ($raw === '') {
            return [];
        }
        $decoded = json_decode($raw, true);
        if (!is_array($decoded)) {
            throw new ApiException('Invalid JSON response from VexPay', 0, $raw);
        }
        return $decoded;
    }

    private function requestRaw(string $method, string $path): string
    {
        return $this->send($method, $path, null, '*/*');
    }

    /**
     * @param array<string,mixed>|null $body
     */
    private function send(string $method, string $path, ?array $body, string $accept): string
    {
        $ch = curl_init($this->baseUrl . $path);
        if ($ch === false) {
            throw new ApiException('Failed to initialise HTTP client');
        }

        $headers = [
            'Authorization: Bearer ' . $this->apiKey,
            'Accept: ' . $accept,
            'User-Agent: vexpay-php/0.1',
        ];

        curl_setopt($ch, CURLOPT_CUSTOMREQUEST, $method);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_TIMEOUT, $this->timeout);

        if ($body !== null) {
            $json = json_encode($body, JSON_THROW_ON_ERROR);
            $headers[] = 'Content-Type: application/json';
            curl_setopt($ch, CURLOPT_POSTFIELDS, $json);
        }
        curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);

        $response = curl_exec($ch);
        if ($response === false) {
            $err = curl_error($ch);
            curl_close($ch);
            throw new ApiException('HTTP request failed: ' . $err);
        }
        $status = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
        curl_close($ch);

        $response = (string) $response;
        if ($status >= 400) {
            throw new ApiException($this->errorMessage($response, $status), $status, $response);
        }
        return $response;
    }

    private function errorMessage(string $body, int $status): string
    {
        $decoded = json_decode($body, true);
        if (is_array($decoded) && isset($decoded['error']) && is_string($decoded['error'])) {
            return 'VexPay API error (' . $status . '): ' . $decoded['error'];
        }
        return 'VexPay API error (' . $status . ')';
    }
}
