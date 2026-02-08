import type {ValueMap} from '@traceviz/client-core';
import {
  Trace,
  dbl,
  int,
  node,
  sec,
  str,
  strs,
  ts,
  valueMap,
} from '@traceviz/client-core';

function traceRenderSettings(): ValueMap {
  return valueMap(
    { key: 'span_width_cat_px', val: int(15) },
    { key: 'span_padding_cat_px', val: int(0) },
    { key: 'category_header_cat_px', val: int(10) },
    { key: 'category_handle_val_px', val: int(10) },
    { key: 'category_padding_cat_px', val: int(3) },
    { key: 'category_margin_val_px', val: int(5) },
    { key: 'category_min_width_cat_px', val: int(15) },
    { key: 'category_base_width_val_px', val: int(40) },
    { key: 'x_axis_render_markers_height_px', val: dbl(24) },
    { key: 'x_axis_render_label_height_px', val: dbl(16) },
  );
}

/**
 * Builds a moderately complex trace with nested categories, spans, subspans,
 * and edges. This mirrors the canonical trace used in Angular tests.
 */
export function buildTestTrace(): Trace<unknown> {
  const rpcNode = node(
    valueMap(
      { key: 'category_defined_id', val: str('x_axis') },
      { key: 'category_display_name', val: str('time from start') },
      { key: 'category_description', val: str('Time from start') },
      { key: 'axis_type', val: str('timestamp') },
      { key: 'axis_min', val: ts(sec(0)) },
      { key: 'axis_max', val: ts(sec(300)) },
    ),
    node(
      valueMap(
        { key: 'trace_node_type', val: int(0) },
        { key: 'category_defined_id', val: str('rpc a') },
        { key: 'category_display_name', val: str('RPC a') },
        { key: 'category_description', val: str('RPC a') },
        { key: 'label_format', val: str('a') },
        { key: 'primary_color', val: str('#888888') },
        { key: 'secondary_color', val: str('#FFFF33') },
      ),
      node(
        valueMap(
          { key: 'trace_node_type', val: int(1) },
          { key: 'trace_start', val: ts(sec(0)) },
          { key: 'trace_end', val: ts(sec(300)) },
          { key: 'rpc', val: str('a') },
          { key: 'label_format', val: str('$(rpc)') },
          { key: 'primary_color', val: str('#888888') },
        ),
        node(
          valueMap(
            { key: 'payload_type', val: str('trace_edge_payload') },
            { key: 'trace_edge_node_id', val: str('a->a/b') },
            { key: 'trace_edge_start', val: ts(sec(0)) },
            { key: 'trace_edge_endpoint_node_ids', val: strs('a/b') },
            { key: 'primary_color', val: str('#888888') },
          ),
        ),
        node(
          valueMap(
            { key: 'payload_type', val: str('trace_edge_payload') },
            { key: 'trace_edge_node_id', val: str('a->a/e') },
            { key: 'trace_edge_start', val: ts(sec(220)) },
            { key: 'trace_edge_endpoint_node_ids', val: strs('a/e') },
            { key: 'primary_color', val: str('#888888') },
          ),
        ),
      ),
      node(
        valueMap(
          { key: 'trace_node_type', val: int(0) },
          { key: 'category_defined_id', val: str('rpc b') },
          { key: 'category_display_name', val: str('RPC a/b') },
          { key: 'category_description', val: str('RPC a/b') },
          { key: 'label_format', val: str('a/b') },
          { key: 'primary_color', val: str('#888888') },
          { key: 'secondary_color', val: str('#FFFF33') },
        ),
        node(
          valueMap(
            { key: 'trace_node_type', val: int(1) },
            { key: 'trace_start', val: ts(sec(0)) },
            { key: 'trace_end', val: ts(sec(180)) },
            { key: 'rpc', val: str('b') },
            { key: 'label_format', val: str('$(rpc)') },
            { key: 'primary_color', val: str('#888888') },
          ),
          node(
            valueMap(
              { key: 'payload_type', val: str('trace_edge_payload') },
              { key: 'trace_edge_node_id', val: str('a/b') },
              { key: 'trace_edge_start', val: ts(sec(0)) },
              { key: 'trace_edge_endpoint_node_ids', val: strs() },
              { key: 'primary_color', val: str('#888888') },
            ),
          ),
          node(
            valueMap(
              { key: 'payload_type', val: str('trace_edge_payload') },
              { key: 'trace_edge_node_id', val: str('a/b->a/b/c') },
              { key: 'trace_edge_start', val: ts(sec(20)) },
              { key: 'trace_edge_endpoint_node_ids', val: strs('a/b/c') },
              { key: 'primary_color', val: str('#888888') },
            ),
          ),
          node(
            valueMap(
              { key: 'payload_type', val: str('trace_edge_payload') },
              { key: 'trace_edge_node_id', val: str('a/b->a/b/d') },
              { key: 'trace_edge_start', val: ts(sec(140)) },
              { key: 'trace_edge_endpoint_node_ids', val: strs('a/b/d') },
              { key: 'primary_color', val: str('#888888') },
            ),
          ),
        ),
        node(
          valueMap(
            { key: 'trace_node_type', val: int(0) },
            { key: 'category_defined_id', val: str('rpc c') },
            { key: 'category_display_name', val: str('RPC a/b/c') },
            { key: 'category_description', val: str('RPC a/b/c') },
            { key: 'label_format', val: str('a/b/c') },
            { key: 'primary_color', val: str('#888888') },
            { key: 'secondary_color', val: str('#FFFF33') },
          ),
          node(
            valueMap(
              { key: 'trace_node_type', val: int(1) },
              { key: 'trace_start', val: ts(sec(20)) },
              { key: 'trace_end', val: ts(sec(120)) },
              { key: 'rpc', val: str('c') },
              { key: 'label_format', val: str('$(rpc)') },
              { key: 'primary_color', val: str('#888888') },
            ),
            node(
              valueMap(
                { key: 'payload_type', val: str('trace_edge_payload') },
                { key: 'trace_edge_node_id', val: str('a/b/c') },
                { key: 'trace_edge_start', val: ts(sec(20)) },
                { key: 'trace_edge_endpoint_node_ids', val: strs() },
                { key: 'primary_color', val: str('#888888') },
              ),
            ),
          ),
        ),
        node(
          valueMap(
            { key: 'trace_node_type', val: int(0) },
            { key: 'category_defined_id', val: str('rpc d') },
            { key: 'category_display_name', val: str('RPC a/b/d') },
            { key: 'category_description', val: str('RPC a/b/d') },
            { key: 'label_format', val: str('a/b/d') },
            { key: 'primary_color', val: str('#888888') },
            { key: 'secondary_color', val: str('#FFFF33') },
          ),
          node(
            valueMap(
              { key: 'trace_node_type', val: int(1) },
              { key: 'trace_start', val: ts(sec(140)) },
              { key: 'trace_end', val: ts(sec(160)) },
              { key: 'rpc', val: str('d') },
              { key: 'label_format', val: str('$(rpc)') },
              { key: 'primary_color', val: str('#888888') },
            ),
            node(
              valueMap(
                { key: 'payload_type', val: str('trace_edge_payload') },
                { key: 'trace_edge_node_id', val: str('a/b/d') },
                { key: 'trace_edge_start', val: ts(sec(140)) },
                { key: 'trace_edge_endpoint_node_ids', val: strs() },
                { key: 'primary_color', val: str('#888888') },
              ),
            ),
          ),
        ),
      ),
      node(
        valueMap(
          { key: 'trace_node_type', val: int(0) },
          { key: 'category_defined_id', val: str('rpc e') },
          { key: 'category_display_name', val: str('RPC a/e') },
          { key: 'category_description', val: str('RPC a/e') },
          { key: 'label_format', val: str('a/e') },
          { key: 'primary_color', val: str('#888888') },
          { key: 'secondary_color', val: str('#FFFF33') },
        ),
        node(
          valueMap(
            { key: 'trace_node_type', val: int(1) },
            { key: 'trace_start', val: ts(sec(220)) },
            { key: 'trace_end', val: ts(sec(280)) },
            { key: 'rpc', val: str('e') },
            { key: 'label_format', val: str('$(rpc)') },
            { key: 'primary_color', val: str('#888888') },
          ),
          node(
            valueMap(
              { key: 'payload_type', val: str('trace_edge_payload') },
              { key: 'trace_edge_node_id', val: str('a/e') },
              { key: 'trace_edge_start', val: ts(sec(220)) },
              { key: 'trace_edge_endpoint_node_ids', val: strs() },
              { key: 'primary_color', val: str('#888888') },
            ),
          ),
          node(
            valueMap(
              { key: 'payload_type', val: str('trace_edge_payload') },
              { key: 'trace_edge_node_id', val: str('a/e->a/e/a') },
              { key: 'trace_edge_start', val: ts(sec(240)) },
              { key: 'trace_edge_endpoint_node_ids', val: strs('a/e/a') },
              { key: 'primary_color', val: str('#888888') },
            ),
          ),
        ),
        node(
          valueMap(
            { key: 'trace_node_type', val: int(0) },
            { key: 'category_defined_id', val: str('rpc a') },
            { key: 'category_display_name', val: str('RPC a/e/a') },
            { key: 'category_description', val: str('RPC a/e/a') },
            { key: 'label_format', val: str('a/e/a') },
            { key: 'primary_color', val: str('#888888') },
            { key: 'secondary_color', val: str('#FFFF33') },
          ),
          node(
            valueMap(
              { key: 'trace_node_type', val: int(1) },
              { key: 'trace_start', val: ts(sec(240)) },
              { key: 'trace_end', val: ts(sec(250)) },
              { key: 'rpc', val: str('a') },
              { key: 'label_format', val: str('$(rpc)') },
              { key: 'primary_color', val: str('#888888') },
            ),
            node(
              valueMap(
                { key: 'payload_type', val: str('trace_edge_payload') },
                { key: 'trace_edge_node_id', val: str('a/e/a') },
                { key: 'trace_edge_start', val: ts(sec(240)) },
                { key: 'trace_edge_endpoint_node_ids', val: strs() },
                { key: 'primary_color', val: str('#888888') },
              ),
            ),
            node(
              valueMap(
                { key: 'trace_node_type', val: int(2) },
                { key: 'trace_start', val: ts(sec(240)) },
                { key: 'trace_end', val: ts(sec(250)) },
                { key: 'state', val: str('local') },
                { key: 'label_format', val: str('$(state)') },
                { key: 'primary_color', val: str('#888888') },
              ),
            ),
          ),
        ),
      ),
    ),
  );

  return Trace.fromNode(rpcNode.with(traceRenderSettings()));
}
