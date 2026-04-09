package tui

import "testing"

func TestRenderBodyWithDimensionsCachesStableFrames(t *testing.T) {
	model := benchmarkModel(overviewSection)
	sidebarWidth, bodyHeight, mainWidth := model.bodyDimensions()

	first := renderBodyWithDimensions(model, sidebarWidth, bodyHeight, mainWidth)
	if first == "" {
		t.Fatal("expected initial body render output")
	}
	if model.renderCache == nil || model.renderCache.bodyKey == "" {
		t.Fatalf("expected body cache to be populated, got %+v", model.renderCache)
	}

	model.renderCache.bodyValue = "cached body"
	cached := renderBodyWithDimensions(model, sidebarWidth, bodyHeight, mainWidth)
	if cached != "cached body" {
		t.Fatalf("expected cached body render, got %q", cached)
	}

	model.contentVersion++
	updated := renderBodyWithDimensions(model, sidebarWidth, bodyHeight, mainWidth)
	if updated == "cached body" {
		t.Fatalf("expected content version change to invalidate body cache, got %q", updated)
	}
}

func TestRenderBodyWithDimensionsWithoutRenderCache(t *testing.T) {
	model := benchmarkModel(servicesSection)
	model.renderCache = nil

	sidebarWidth, bodyHeight, mainWidth := model.bodyDimensions()
	rendered := renderBodyWithDimensions(model, sidebarWidth, bodyHeight, mainWidth)
	if rendered == "" {
		t.Fatal("expected body render without cache")
	}
}

func TestModelViewCachesStableFrames(t *testing.T) {
	model := benchmarkModel(overviewSection)

	first := model.View().Content
	if first == "" {
		t.Fatal("expected initial view output")
	}
	if model.renderCache == nil || !model.renderCache.viewSet {
		t.Fatalf("expected view cache to be populated, got %+v", model.renderCache)
	}

	model.renderCache.viewValue = "cached frame"
	cached := model.View().Content
	if cached != "cached frame" {
		t.Fatalf("expected cached view render, got %q", cached)
	}

	model.contentVersion++
	updated := model.View().Content
	if updated == "cached frame" {
		t.Fatalf("expected content change to invalidate view cache, got %q", updated)
	}
}

func TestModelViewSkipsCacheForBanners(t *testing.T) {
	model := benchmarkModel(overviewSection)

	_ = model.View()
	model.renderCache.viewValue = "cached frame"
	model.banner = &actionBanner{Status: "warn", Message: "watch out"}

	rendered := model.View().Content
	if rendered == "cached frame" {
		t.Fatalf("expected banner view to bypass cache, got %q", rendered)
	}
}
