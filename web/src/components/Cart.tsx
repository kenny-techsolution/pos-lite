export function Cart({ items }: { items: { sku: string; qty: number }[] }) {
  return <ul>{items.map(i => <li key={i.sku}>{i.sku} × {i.qty}</li>)}</ul>;
}
