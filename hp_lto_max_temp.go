package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

// ioctl constants for SCSI generic (SG) operations
const (
	SG_IO             = 0x2285
	SCSI_SEND_DIAG    = 0x1D // SCSI "Send Diagnostic" command opcode
	SG_DXFER_TO_DEV   = -2   // Direction of data transfer (to device)
	SG_DXFER_FROM_DEV = 1    // Direction of data transfer (from device)
)

// SCSI command and data lengths
const (
	SCSI_CMD_LEN = 6
	DATA_OUT_LEN = 8
	DATA_IN_LEN  = 68 // Set to capture enough bytes for the raw output
)

// SCSI command sequences
var (
	sendDiagnosticCmd = [SCSI_CMD_LEN]byte{
		0x1D, // Operation Code: SEND DIAGNOSTIC command (0x1D)
		0x10, // Set the PF (Page Format) bit to indicate a parameter list is provided
		0x00, // Reserved byte (must be set to 0)
		0x00, // Reserved byte (must be set to 0)
		0x08, // Parameter list length: length of the following data (8 bytes)
		0x00, // Control byte (set to 0 for default behavior)
	}

	sendDiagnosticDataOut = [DATA_OUT_LEN]byte{
		0x93, // Diagnostic Parameter, typically specific to the device's diagnostic function
		0x00, // Reserved byte (must be set to 0)
		0x00, // Reserved byte (must be set to 0)
		0x04, // Additional diagnostic parameter specific to the command
		0x00, // Reserved byte (must be set to 0)
		0x00, // Reserved byte (must be set to 0)
		0x20, // Additional diagnostic parameter, device-specific
		0x2A, // Additional diagnostic parameter, device-specific
	}

	receiveDiagnosticCmd = [SCSI_CMD_LEN]byte{
		0x1C, // Operation Code: RECEIVE DIAGNOSTIC RESULTS command (0x1C)
		0x01, // Specifies the page code (0x01) to select the required diagnostic information
		0x93, // Additional parameter specifying the diagnostic result page code
		0x00, // Reserved byte (must be set to 0)
		0x44, // Allocation length: size of the expected data to be received (68 bytes)
		0x00, // Control byte (set to 0 for default behavior)
	}
)

// SG_IO_Header structure for sending SCSI commands
type SG_IO_Header struct {
	interface_id    int32   // Identifier for the interface, typically set to 'S' for SCSI
	dxfer_direction int32   // Data transfer direction: -2 for host to device, 1 for device to host
	cmd_len         uint8   // Length of the SCSI command descriptor block (CDB) in bytes
	mx_sb_len       uint8   // Maximum length of the sense buffer, used for error reporting
	iovec_count     uint16  // Count for scatter-gather lists, set to 0 if not used
	dxfer_len       uint32  // Length of the data to be transferred in bytes
	dxferp          uintptr // Pointer to the data buffer for data transfer (input or output)
	cmdp            uintptr // Pointer to the command descriptor block (CDB)
	sbp             uintptr // Pointer to the sense buffer, which stores error information
	timeout         uint32  // Command timeout in milliseconds
	flags           uint32  // Additional flags for command execution (e.g., blocking, etc.)
	pack_id         int32   // Packet ID used to track the command
	usr_ptr         uintptr // User-defined pointer, often used for additional data tracking
	status          uint8   // Status byte returned from the device, indicating success or error
	masked_status   uint8   // Internal masked status, used by the driver
	msg_status      uint8   // Message byte returned by the device
	sb_len_wr       uint8   // Actual length of the sense buffer written by the device
	host_status     uint16  // Host-specific status code, set by the driver
	driver_status   uint16  // Driver-specific status code, set by the driver
	resid           int32   // Residual byte count, indicating remaining data not transferred
	duration        uint32  // Duration the command took to execute, in milliseconds
	info            uint32  // Additional information about the command, such as retries or errors
}

var verbose bool

func main() {
	
	// Define and parse the flags
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [--verbose] <scsi_device>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "Example: %s --verbose /dev/sg4\n", os.Args[0])
	}
	flag.Parse()

	// Check if --help flag is specified, show usage and exit with zero status code
	if len(os.Args) > 1 && os.Args[1] == "--help" {
		flag.Usage()
		os.Exit(0)
	}

	// Check if a device argument is provided
	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	device := flag.Args()[0]

	file, err := os.OpenFile(device, os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("Failed to open device %s: %v\n", device, err)
		os.Exit(1)
	}
	defer file.Close()

	// Step 1: Send SEND DIAGNOSTIC command
	if verbose {
		fmt.Printf("Sending SEND DIAGNOSTIC command with cmd=%s and dataOut=%s\n", formatBytes(sendDiagnosticCmd[:]), formatBytes(sendDiagnosticDataOut[:]))
	}
	if err := sendScsiCommand(file, sendDiagnosticCmd[:], sendDiagnosticDataOut[:], nil, SG_DXFER_TO_DEV, 60*time.Second); err != nil {
		fmt.Printf("Failed to send SEND DIAGNOSTIC command: %v\n", err)
		os.Exit(1)
	}
	if verbose {
		fmt.Println("SEND DIAGNOSTIC command sent successfully.")
	}

	// Step 2: Send RECEIVE DIAGNOSTIC command to retrieve raw diagnostic data
	dataIn := make([]byte, DATA_IN_LEN)
	if verbose {
		fmt.Printf("Sending RECEIVE DIAGNOSTIC command with cmd=%s\n", formatBytes(receiveDiagnosticCmd[:]))
	}
	if err := sendScsiCommand(file, receiveDiagnosticCmd[:], nil, dataIn, SG_DXFER_FROM_DEV, 10*time.Second); err != nil {
		fmt.Printf("Failed to send RECEIVE DIAGNOSTIC command: %v\n", err)
		os.Exit(1)
	}
	if verbose {
		fmt.Println("RECEIVE DIAGNOSTIC command sent successfully.")
		fmt.Printf("Raw diagnostic data (hex): %s\n", hex.EncodeToString(dataIn))
	}

	// Step 3: Extract and decode temperature data
	temperatureHex, err := extractAndConvertTemperature(dataIn)
	if err != nil {
		fmt.Printf("Error extracting temperature: %v\n", err)
		os.Exit(1)
	}
	if verbose {
		fmt.Printf("Max temperature (hex): %s\n", temperatureHex)
	}

	// Step 4: Convert hex to decimal
	temperatureDecimal, err := hexToDecimal(temperatureHex)
	if err != nil {
		fmt.Printf("Error converting hex to decimal: %v\n", err)
		os.Exit(1)
	}
	if verbose {
		fmt.Printf("Max temperature (decimal): %d\n", temperatureDecimal)
	}

	// Step 5: Calculate temperature in °C
	temperatureC := float64(temperatureDecimal) / 256
	fmt.Printf("Max temperature since cartridge loaded: %.1f°C\n", temperatureC)
}

// formatBytes is a helper function to format byte slices as hexadecimal strings for printing
func formatBytes(data []byte) string {
	formatted := ""
	for i, b := range data {
		if i > 0 {
			formatted += " "
		}
		formatted += fmt.Sprintf("0x%02X", b)
	}
	return formatted
}

// sendScsiCommand sends a SCSI command to a device using the SG_IO ioctl
func sendScsiCommand(file *os.File, cmd []byte, dataOut []byte, dataIn []byte, direction int32, timeout time.Duration) error {
	// Allocate sense buffer for error reporting
	sense := make([]byte, 32)

	// Prepare the SG_IO_Header
	header := SG_IO_Header{
		interface_id:    'S',
		dxfer_direction: direction,
		cmd_len:         uint8(len(cmd)),
		mx_sb_len:       uint8(len(sense)),
		dxfer_len:       uint32(len(dataOut) + len(dataIn)),
		cmdp:            uintptr(unsafe.Pointer(&cmd[0])),
		sbp:             uintptr(unsafe.Pointer(&sense[0])),
		timeout:         uint32(timeout / time.Millisecond), // Convert timeout to milliseconds
	}

	// Set up data transfer pointers if needed
	if len(dataOut) > 0 {
		header.dxferp = uintptr(unsafe.Pointer(&dataOut[0]))
	}
	if len(dataIn) > 0 {
		header.dxferp = uintptr(unsafe.Pointer(&dataIn[0]))
	}

	// Execute ioctl command
	if verbose {
		fmt.Printf("Executing ioctl with cmd=%s, dataOut=%s, dataInLen=%d\n", formatBytes(cmd), formatBytes(dataOut), len(dataIn))
	}
	if err := ioctl(int(file.Fd()), SG_IO, uintptr(unsafe.Pointer(&header))); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	// Check for SCSI status success and output sense data if failure occurs
	if header.status != 0 {
		if verbose {
			fmt.Printf("Command failed with status: %d\n", header.status)
			fmt.Printf("Sense data: %s\n", hex.EncodeToString(sense))
		}
		return fmt.Errorf("command failed with status: %d", header.status)
	}

	return nil
}

// extractAndConvertTemperature extracts bytes 22-29, converts each pair of ASCII hex characters, and returns the decoded hex string
func extractAndConvertTemperature(data []byte) (string, error) {
	if len(data) < 30 {
		return "", fmt.Errorf("data length is too short")
	}

	// Extract bytes 22-29 and convert each pair of ASCII-encoded hex characters to their byte value
	var tempHex string
	for i := 22; i < 30; i += 2 {
		// Combine two ASCII characters (e.g., "30" -> "0")
		bytePair := string(data[i : i+2])
		byteVal, err := strconv.ParseUint(bytePair, 16, 8)
		if err != nil {
			return "", fmt.Errorf("error parsing byte pair %s: %v", bytePair, err)
		}
		tempHex += fmt.Sprintf("%X", byteVal)
	}
	return tempHex, nil
}

// hexToDecimal converts a hexadecimal string to a decimal integer
func hexToDecimal(hexStr string) (int64, error) {
	return strconv.ParseInt(hexStr, 16, 64)
}

// ioctl function to send commands to the device
func ioctl(fd int, request int, arg uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(request), arg)
	if errno != 0 {
		return errno
	}
	return nil
}
