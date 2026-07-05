<?php

declare(strict_types=1);

namespace VexPay;

/**
 * An immutable view of a VexPay invoice.
 *
 * Monetary values (amount, received, rate) are exact decimal strings — never
 * cast them to float for comparisons or accounting.
 */
final class Invoice
{
    /**
     * @param array<string,string> $metadata
     * @param array<string,mixed>  $raw       The full decoded API payload.
     */
    public function __construct(
        public readonly string $id,
        public readonly string $merchantId,
        public readonly string $chain,
        public readonly string $asset,
        public readonly string $amount,
        public readonly string $status,
        public readonly string $received,
        public readonly int $confirmations,
        public readonly int $requiredConfirmations,
        public readonly string $receiveAddress,
        public readonly string $paymentUri,
        public readonly string $txHash,
        public readonly string $fiatCurrency,
        public readonly string $fiatAmount,
        public readonly string $rate,
        public readonly array $metadata,
        public readonly ?string $createdAt,
        public readonly ?string $expiresAt,
        public readonly ?string $paidAt,
        public readonly ?string $updatedAt,
        public readonly array $raw,
    ) {
    }

    /**
     * Build an Invoice from a decoded API response array.
     *
     * @param array<string,mixed> $data
     */
    public static function fromArray(array $data): self
    {
        return new self(
            id: (string) ($data['id'] ?? ''),
            merchantId: (string) ($data['merchant_id'] ?? ''),
            chain: (string) ($data['chain'] ?? ''),
            asset: (string) ($data['asset'] ?? ''),
            amount: (string) ($data['amount'] ?? '0'),
            status: (string) ($data['status'] ?? ''),
            received: (string) ($data['received'] ?? '0'),
            confirmations: (int) ($data['confirmations'] ?? 0),
            requiredConfirmations: (int) ($data['required_confirmations'] ?? 0),
            receiveAddress: (string) ($data['receive_address'] ?? ''),
            paymentUri: (string) ($data['payment_uri'] ?? ''),
            txHash: (string) ($data['tx_hash'] ?? ''),
            fiatCurrency: (string) ($data['fiat_currency'] ?? ''),
            fiatAmount: (string) ($data['fiat_amount'] ?? ''),
            rate: (string) ($data['rate'] ?? ''),
            metadata: array_map('strval', $data['metadata'] ?? []),
            createdAt: isset($data['created_at']) ? (string) $data['created_at'] : null,
            expiresAt: isset($data['expires_at']) ? (string) $data['expires_at'] : null,
            paidAt: isset($data['paid_at']) ? (string) $data['paid_at'] : null,
            updatedAt: isset($data['updated_at']) ? (string) $data['updated_at'] : null,
            raw: $data,
        );
    }

    /** True once the invoice is fully paid (paid or overpaid). */
    public function isPaid(): bool
    {
        return $this->status === 'paid' || $this->status === 'overpaid';
    }

    /** True once the invoice can no longer change (paid, overpaid or expired). */
    public function isTerminal(): bool
    {
        return $this->isPaid() || $this->status === 'expired';
    }
}
