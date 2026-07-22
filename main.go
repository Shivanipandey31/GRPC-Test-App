package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"

	pb "grpc-dedup-test/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedDedupServiceServer
}

// ── Group A: GetItem ─────────────────────────────────────────────────────────
// Same nested structure for every id; only scalar values differ.
// All calls must dedup to a single test case.
func (s *server) GetItem(_ context.Context, req *pb.GetItemReq) (*pb.Item, error) {
	id := req.GetId()
	r := rand.New(rand.NewSource(int64(id)))
	adjectives := []string{"Turbo", "Nano", "Ultra", "Hyper", "Mega"}
	nouns := []string{"Widget", "Gadget", "Gizmo", "Thingamajig", "Doohickey"}
	countries := []string{"Germany", "Japan", "USA", "Canada", "Brazil"}

	return &pb.Item{
		Id:      id,
		Name:    fmt.Sprintf("%s %s #%d", adjectives[r.Intn(len(adjectives))], nouns[r.Intn(len(nouns))], id),
		Price:   float64(r.Intn(9900)+100) / 100.0,
		InStock: id%2 == 0,
		Tags:    []string{"electronics", fmt.Sprintf("tier-%d", id%3+1), "in-season"},
		Attributes: map[string]string{
			"color":  []string{"red", "blue", "green", "black"}[r.Intn(4)],
			"weight": fmt.Sprintf("%dg", r.Intn(500)+50),
			"sku":    fmt.Sprintf("SKU-%04d", id*7+13),
		},
		Detail: &pb.ItemDetail{
			Description: fmt.Sprintf("Premium product #%d with advanced features.", id),
			StockCount:  int32(r.Intn(200) + 1),
			Supplier: &pb.Supplier{
				Name:    fmt.Sprintf("Supplier Corp %d", id%5+1),
				Country: countries[r.Intn(len(countries))],
				Contact: fmt.Sprintf("contact%d@supplier.example", id),
			},
		},
	}, nil
}

// ── Group B: GetWidget ───────────────────────────────────────────────────────
// mode="full"    → config nested message IS present on wire  (TC-1)
// mode="minimal" → config is nil, absent from wire           (TC-2)
func (s *server) GetWidget(_ context.Context, req *pb.GetWidgetReq) (*pb.Widget, error) {
	w := &pb.Widget{
		Id:   fmt.Sprintf("widget-%s", req.GetMode()),
		Type: "UI_COMPONENT",
	}
	if req.GetMode() == "full" {
		w.Config = &pb.WidgetConfig{
			Theme: "dark",
			Size:  42,
			Style: &pb.WidgetStyle{
				Color:  "#1a1a2e",
				Bold:   true,
				Weight: 700,
			},
		}
	}
	// mode="minimal" → Config stays nil → field 3 absent from wire
	return w, nil
}

// ── Group C: RiskyCall ───────────────────────────────────────────────────────
// should_fail=false → grpc-status=0 in trailer                (TC-3)
// should_fail=true  → grpc-status=3 (InvalidArgument)         (TC-4)
func (s *server) RiskyCall(_ context.Context, req *pb.RiskyReq) (*pb.RiskyResp, error) {
	if req.GetShouldFail() {
		return nil, status.Error(codes.InvalidArgument, "intentional failure for dedup edge case testing")
	}
	return &pb.RiskyResp{Result: "success"}, nil
}

// ── Group D: GetReport ───────────────────────────────────────────────────────
// Deeply nested response. Values change per period but structure is identical.
// Q1, Q2, Q3 all dedup to a single test case.
func (s *server) GetReport(_ context.Context, req *pb.ReportReq) (*pb.Report, error) {
	period := req.GetPeriod()
	seed := int64(0)
	for _, c := range period {
		seed += int64(c)
	}
	r := rand.New(rand.NewSource(seed))

	total := int32(r.Intn(500) + 100)
	passed := int32(r.Intn(int(total)))
	failed := total - passed

	return &pb.Report{
		Id:     fmt.Sprintf("report-%s-%04d", period, r.Intn(9999)),
		Period: period,
		Summary: &pb.ReportSummary{
			Total:  total,
			Passed: passed,
			Failed: failed,
			Meta: &pb.ReportMeta{
				GeneratedBy: "dedup-test-server",
				Version:     fmt.Sprintf("v1.%d.0", r.Intn(10)),
				Env: &pb.Environment{
					Name:   "staging",
					Region: []string{"us-east-1", "eu-west-1", "ap-south-1"}[r.Intn(3)],
					Labels: map[string]string{
						"team":    "platform",
						"project": "keploy-dedup",
						"period":  period,
					},
				},
			},
		},
		Sections: []*pb.Section{
			{
				Title: "Integration Tests",
				Rows: []*pb.Row{
					{Key: "grpc",  Value: fmt.Sprintf("%d", r.Intn(50)+10), Score: int32(r.Intn(100))},
					{Key: "http",  Value: fmt.Sprintf("%d", r.Intn(80)+20), Score: int32(r.Intn(100))},
					{Key: "kafka", Value: fmt.Sprintf("%d", r.Intn(20)+5),  Score: int32(r.Intn(100))},
				},
			},
			{
				Title: "Unit Tests",
				Rows: []*pb.Row{
					{Key: "pkg/matcher", Value: fmt.Sprintf("%d", r.Intn(200)+50), Score: int32(r.Intn(100))},
					{Key: "pkg/agent",  Value: fmt.Sprintf("%d", r.Intn(100)+30), Score: int32(r.Intn(100))},
				},
			},
		},
		Counters: map[string]int32{
			"flaky":   int32(r.Intn(5)),
			"skipped": int32(r.Intn(10)),
			"timeout": int32(r.Intn(3)),
		},
	}, nil
}

// ── Group E: GetVariant ──────────────────────────────────────────────────────
// oneof puts a different field number on the wire for each variant.
// "text"   → field 1 (string, wire type 2)  (TC-5)
// "number" → field 2 (int32,  wire type 0)  (TC-6)
// "flag"   → field 3 (bool,   wire type 0)  (TC-7)
func (s *server) GetVariant(_ context.Context, req *pb.VariantReq) (*pb.VariantResp, error) {
	resp := &pb.VariantResp{Label: fmt.Sprintf("variant-%s", req.GetType())}
	switch req.GetType() {
	case "text":
		resp.Payload = &pb.VariantResp_TextValue{TextValue: "hello from dedup test"}
	case "number":
		resp.Payload = &pb.VariantResp_NumberValue{NumberValue: 42}
	case "flag":
		resp.Payload = &pb.VariantResp_FlagValue{FlagValue: true}
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown variant type %q", req.GetType())
	}
	return resp, nil
}

// ── Group F: ListItems ───────────────────────────────────────────────────────
// include_items=true  → repeated field 1 has elements → field 1 on wire  (TC-8)
// include_items=false → items is empty → field 1 absent from wire         (TC-9)
func (s *server) ListItems(_ context.Context, req *pb.ListReq) (*pb.ListResp, error) {
	if req.GetIncludeItems() {
		return &pb.ListResp{
			Items: []*pb.ListItem{
				{Id: "item-1", Name: "Alpha Widget",  Kind: "physical"},
				{Id: "item-2", Name: "Beta Gadget",   Kind: "digital"},
				{Id: "item-3", Name: "Gamma Gizmo",   Kind: "physical"},
			},
			Total: 3,
		}, nil
	}
	// Empty slice → field 1 absent on wire → different structural hash
	return &pb.ListResp{Total: 0}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterDedupServiceServer(grpcServer, &server{})
	reflection.Register(grpcServer)

	log.Println("grpc-dedup-test listening on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
