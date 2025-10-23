import asyncio
import websockets
import pyautogui
import base64
import json
from io import BytesIO

# Set of connected clients
connected_clients = set()

# Function to handle each client connection
async def handle_client(websocket):
    # Add the new client to the set of connected clients
    connected_clients.add(websocket)
    try:
        # Listen for messages from the client
        async for message in websocket:
            # Parse the message as JSON
            data = json.loads(message)
            print(data)
            
            # Handle the message based on its type
            if data['type'] == 'mouse':
                # Move the mouse to the specified position
                pyautogui.moveTo(data['x'], data['y'])
            elif data['type'] == 'down':
                x = int(data['x'])
                y = int(data['y'])
                print(x, y)
                # Press the specified mouse button
                pyautogui.mouseDown(button=data['button'], x=x, y=y)
            elif data['type'] == 'up':
                x = int(data['x'])
                y = int(data['y'])
                # Press the specified mouse button
                pyautogui.mouseUp(button=data['button'], x=x, y=y)
            elif data['type'] == 'key':
                # Press the specified key
                pyautogui.press(data['key'])
    except websockets.exceptions.ConnectionClosed:
        pass
    finally:
        # Remove the client from the set of connected clients
        connected_clients.remove(websocket)

async def screenshot_loop():
    while True:
        # Take a screenshot
        screenshot = pyautogui.screenshot()
        
        # Convert to PNG format for better compression
        buffer = BytesIO()
        screenshot.save(buffer, format='WEBP', quality=10)
        screenshot_bytes = buffer.getvalue()
        screenshot_base64 = base64.b64encode(screenshot_bytes).decode('utf-8')
        
        # Prepare message
        message = json.dumps({
            'type': 'screenshot',
            'data': screenshot_base64,
            'width': screenshot.width,
            'height': screenshot.height,
            'format': 'webp'
        })
        
        # Send to all connected clients safely
        if connected_clients:
            # Create a copy to avoid issues with set changing during iteration
            clients_copy = connected_clients.copy()
            await asyncio.gather(
                *[client.send(message) for client in clients_copy],
                return_exceptions=True
            )
        
        # Wait 1 second before next screenshot (non-blocking)
        await asyncio.sleep(.1)

# Main function to start the WebSocket server
async def main():
    server = await websockets.serve(handle_client, 'localhost', 8080)
    
    # Run screenshot loop as an async task
    screenshot_task = asyncio.create_task(screenshot_loop())
    
    print("WebSocket server started on ws://localhost:8080")
    
    await server.wait_closed()

# Run the server
if __name__ == "__main__":
    pyautogui.FAILSAFE = False
    asyncio.run(main())