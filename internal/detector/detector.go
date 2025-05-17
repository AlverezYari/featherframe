package detector

import (
	"bufio"
	"fmt"
	"image"
	"os"

	"gocv.io/x/gocv"
)

// Detection represents one object prediction result.
type Detection struct {
	ClassID    int
	Label      string
	Confidence float32
	Box        image.Rectangle
}

// Detector wraps a gocv.Net for running inference.
type Detector struct {
	net           gocv.Net
	labels        []string
	inputWidth    int
	inputHeight   int
	confThreshold float32
}

// New returns a Detector using the provided ONNX model and label file.
func New(modelPath, labelPath string, inputW, inputH int, conf float32) (*Detector, error) {
	net := gocv.ReadNet(modelPath, "")
	if net.Empty() {
		return nil, fmt.Errorf("failed to load model at %s", modelPath)
	}

	labels, err := loadLabels(labelPath)
	if err != nil {
		return nil, err
	}

	return &Detector{
		net:           net,
		labels:        labels,
		inputWidth:    inputW,
		inputHeight:   inputH,
		confThreshold: conf,
	}, nil
}

// Close releases resources used by the detector.
func (d *Detector) Close() {
	if !d.net.Empty() {
		d.net.Close()
	}
}

// Detect runs inference on the provided Mat and returns detections.
// TODO: adapt this to the chosen model's output format.
func (d *Detector) Detect(img gocv.Mat) ([]Detection, error) {
	blob := gocv.BlobFromImage(img, 1/255.0, image.Pt(d.inputWidth, d.inputHeight), gocv.NewScalar(0, 0, 0, 0), true, false)
	defer blob.Close()

	d.net.SetInput(blob, "")

	preds := d.net.Forward("")
	defer preds.Close()

	// Parsing model output is model-specific. Implementation left as TODO.
	// Returning empty slice for now.
	return nil, nil
}

func loadLabels(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open labels: %w", err)
	}
	defer f.Close()

	var labels []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		labels = append(labels, scanner.Text())
	}
	return labels, scanner.Err()
}
